package ingestor

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"japan_data_project/internal/domain/model"
	"japan_data_project/internal/platform/config"
	"japan_data_project/internal/platform/db"
	xlog "japan_data_project/internal/platform/logger"
	"japan_data_project/internal/platform/xerror"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultInputDir         = "data/data_downloads/utf8"
	runStatusOK             = "success"
	runStatusWarn           = "partial_failed"
	runStatusFail           = "failed"
	unknownOriginCode       = "UNKNOWN"
	unknownOriginName       = "미상"
	maxFileTxnRetry         = 3
	ingestorLockKey         = int64(90420260408)
	defaultFileLogBatchSize = 500
)

type options struct {
	inputDir         string
	batchSize        int
	fileLogBatchSize int
	parseWorker      int
	parsedQueue      int
	processedQueue   int
	runType          string
	ext              string
	failOnFile       bool
}

type parsedRow struct {
	TradeDate    time.Time
	WeekdayJA    string
	MarketCode   string
	MarketName   string
	ItemCode     string
	ItemName     string
	OriginCode   string
	OriginName   string
	Grade        string
	Class        string
	ProductName  string
	UnitWeight   string
	ItemTotalTon *float64
	ArrivalTon   *float64
	PriceHighYen *int
	PriceMidYen  *int
	PriceLowYen  *int
	TrendLabel   *string
	SourceFile   string
	SourceRowNo  int
	IngestedAt   time.Time
}

type fileResult struct {
	RelativePath string
	FileHash     string
	RowsTotal    int
	RowsOK       int
	RowsError    int
	Status       string
}

type dimCache struct {
	market map[string]uint
	item   map[string]uint
	origin map[string]uint
	grade  map[string]uint
}

type parsedFile struct {
	result fileResult
	rows   []parsedRow
}

type dateGroupJob struct {
	dateKey string
	files   []string
}

type parsedGroup struct {
	dateKey  string
	parsed   []parsedFile
	results  []fileResult
	parseErr error
}

type processedGroup struct {
	dateKey  string
	results  []fileResult
	groupErr error
}

func Run(args []string) error {
	cfg := config.Load()
	logger := xlog.New(cfg)
	ctx := context.Background()

	opts, err := parseFlags(args)
	if err != nil {
		return err
	}

	gormDB, err := db.OpenGorm(cfg)
	if err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to open gorm", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to get sql db", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.Ping(); err != nil {
		return xerror.Wrap(xerror.CodeDB, "db ping failed", err)
	}
	if err := db.AutoMigrate(gormDB); err != nil {
		return xerror.Wrap(xerror.CodeDB, "auto migrate failed", err)
	}

	pool, err := pgxpool.New(ctx, buildPGXConnString(cfg))
	if err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to open pgx pool", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return xerror.Wrap(xerror.CodeDB, "pgx pool ping failed", err)
	}
	releaseLock, err := acquireIngestorLock(ctx, pool)
	if err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to acquire ingestor lock", err)
	}
	defer releaseLock()

	files, err := collectFiles(opts.inputDir, opts.ext)
	if err != nil {
		return fmt.Errorf("failed to collect files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", opts.inputDir)
	}
	dateKeys, grouped := groupFilesByDate(files, opts.inputDir)

	run := model.IngestionRun{
		RunType:   opts.runType,
		StartedAt: time.Now(),
		Status:    "running",
	}
	if err := gormDB.Create(&run).Error; err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to create ingestion run", err)
	}

	logger.Info("ingestor started", "files", len(files), "date_groups", len(dateKeys), "input_dir", opts.inputDir, "batch_size", opts.batchSize, "parse_workers", opts.parseWorker)

	var (
		totalRows   int
		totalOK     int
		totalErr    int
		fileErrors  int
		firstErrMsg string
	)
	pipelineCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pendingFileLogs := make([]model.IngestionFile, 0, opts.fileLogBatchSize*2)
	flushFileLogs := func() {
		if len(pendingFileLogs) == 0 {
			return
		}
		if err := gormDB.CreateInBatches(pendingFileLogs, opts.fileLogBatchSize).Error; err != nil {
			logger.Error("failed to batch record ingestion file logs", "count", len(pendingFileLogs), "error", err)
			if firstErrMsg == "" {
				firstErrMsg = err.Error()
			}
			fileErrors += len(pendingFileLogs)
			if opts.failOnFile {
				cancel()
			}
		}
		pendingFileLogs = pendingFileLogs[:0]
	}

	jobCh := make(chan dateGroupJob, maxInt(opts.parseWorker*4, 32))
	parsedCh := make(chan parsedGroup, maxInt(opts.parsedQueue, 64))
	processedCh := make(chan processedGroup, maxInt(opts.processedQueue, 64))

	var parseWG sync.WaitGroup
	for i := 0; i < opts.parseWorker; i++ {
		parseWG.Add(1)
		go func() {
			defer parseWG.Done()
			for job := range jobCh {
				select {
				case <-pipelineCtx.Done():
					return
				default:
				}
				parsed, parseErr := parseFiles(job.files, opts.inputDir, time.Now())
				results := make([]fileResult, 0, len(parsed))
				for _, pf := range parsed {
					results = append(results, pf.result)
				}
				pg := parsedGroup{
					dateKey:  job.dateKey,
					parsed:   parsed,
					results:  results,
					parseErr: parseErr,
				}
				filesN, failedN, rowsTotalN, rowsOKN, rowsErrN := summarizeResults(results)
				logger.Info(
					"date group queued",
					"date_group", job.dateKey,
					"files", filesN,
					"failed_files", failedN,
					"rows_total", rowsTotalN,
					"rows_ok", rowsOKN,
					"rows_error", rowsErrN,
					"parse_error", parseErr != nil,
				)
				select {
				case <-pipelineCtx.Done():
					return
				case parsedCh <- pg:
				}
			}
		}()
	}

	go func() {
		parseWG.Wait()
		close(parsedCh)
	}()

	go func() {
		for pg := range parsedCh {
			results, err := ingestParsedGroup(pipelineCtx, pool, pg, opts.batchSize)
			out := processedGroup{
				dateKey:  pg.dateKey,
				results:  results,
				groupErr: err,
			}
			select {
			case <-pipelineCtx.Done():
				close(processedCh)
				return
			case processedCh <- out:
			}
		}
		close(processedCh)
	}()

	go func() {
		defer close(jobCh)
		for _, dateKey := range dateKeys {
			job := dateGroupJob{
				dateKey: dateKey,
				files:   grouped[dateKey],
			}
			select {
			case <-pipelineCtx.Done():
				return
			case jobCh <- job:
			}
		}
	}()

	for pg := range processedCh {
		if pg.groupErr != nil && firstErrMsg == "" {
			firstErrMsg = pg.groupErr.Error()
		}
		filesN, failedN, rowsTotalN, rowsOKN, rowsErrN := summarizeResults(pg.results)
		if pg.groupErr != nil || failedN > 0 {
			logger.Error(
				"date group ingestion failed",
				"date_group", pg.dateKey,
				"files", filesN,
				"failed_files", failedN,
				"rows_total", rowsTotalN,
				"rows_ok", rowsOKN,
				"rows_error", rowsErrN,
				"error", pg.groupErr,
			)
		} else {
			logger.Info(
				"date group ingested",
				"date_group", pg.dateKey,
				"files", filesN,
				"rows_total", rowsTotalN,
				"rows_ok", rowsOKN,
				"rows_error", rowsErrN,
			)
		}
		for _, res := range pg.results {
			if res.Status == runStatusFail {
				fileErrors++
			}

			totalRows += res.RowsTotal
			totalOK += res.RowsOK
			totalErr += res.RowsError

			pendingFileLogs = append(pendingFileLogs, model.IngestionFile{
				RunID:     run.ID,
				FilePath:  res.RelativePath,
				FileHash:  res.FileHash,
				RowsTotal: res.RowsTotal,
				RowsOK:    res.RowsOK,
				RowsError: res.RowsError,
				Status:    res.Status,
			})
			if len(pendingFileLogs) >= opts.fileLogBatchSize {
				flushFileLogs()
			}
		}
		if opts.failOnFile && (pg.groupErr != nil || fileErrors > 0) {
			cancel()
		}
	}
	flushFileLogs()

	finishedAt := time.Now()
	finalStatus := runStatusOK
	var errMessage *string
	if fileErrors > 0 {
		if opts.failOnFile {
			finalStatus = runStatusFail
		} else {
			finalStatus = runStatusWarn
		}
		if firstErrMsg != "" {
			errMessage = &firstErrMsg
		}
	}

	if err := gormDB.Model(&model.IngestionRun{}).
		Where("id = ?", run.ID).
		Updates(map[string]any{
			"finished_at":   &finishedAt,
			"status":        finalStatus,
			"error_message": errMessage,
			"updated_at":    finishedAt,
		}).Error; err != nil {
		return xerror.Wrap(xerror.CodeDB, "failed to update ingestion run", err)
	}

	logger.Info("ingestor finished", "run_id", run.ID, "status", finalStatus, "files", len(files), "file_errors", fileErrors, "rows_total", totalRows, "rows_ok", totalOK, "rows_error", totalErr)

	if fileErrors > 0 && opts.failOnFile {
		return fmt.Errorf("ingestion failed: %s", firstErrMsg)
	}
	return nil
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("ingestor", flag.ContinueOnError)
	fs.StringVar(&o.inputDir, "in", defaultInputDir, "input directory containing utf8 csv files")
	fs.IntVar(&o.batchSize, "batch-size", 500, "batch insert size for fact rows")
	fs.IntVar(&o.fileLogBatchSize, "file-log-batch-size", defaultFileLogBatchSize, "batch size for ingestion_files insert")
	fs.IntVar(&o.parseWorker, "parse-workers", 4, "number of csv parse workers that build date-group datasets")
	fs.IntVar(&o.parsedQueue, "parsed-queue-size", 256, "buffer size for parsed date-group queue")
	fs.IntVar(&o.processedQueue, "processed-queue-size", 256, "buffer size for processed result queue")
	fs.StringVar(&o.runType, "run-type", "manual_utf8", "ingestion run type label")
	fs.StringVar(&o.ext, "ext", ".csv", "target file extension")
	fs.BoolVar(&o.failOnFile, "fail-on-file-error", false, "stop and fail run when a file ingestion error occurs")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.batchSize <= 0 {
		return o, errors.New("batch-size must be > 0")
	}
	if o.fileLogBatchSize <= 0 {
		return o, errors.New("file-log-batch-size must be > 0")
	}
	if o.parseWorker <= 0 {
		return o, errors.New("parse-workers must be > 0")
	}
	if o.parsedQueue <= 0 {
		return o, errors.New("parsed-queue-size must be > 0")
	}
	if o.processedQueue <= 0 {
		return o, errors.New("processed-queue-size must be > 0")
	}
	return o, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func acquireIngestorLock(ctx context.Context, pool *pgxpool.Pool) (func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", ingestorLockKey).Scan(&locked); err != nil {
		conn.Release()
		return nil, err
	}
	if !locked {
		conn.Release()
		return nil, errors.New("another ingestor instance is already running")
	}

	return func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", ingestorLockKey)
		conn.Release()
	}, nil
}

func summarizeResults(results []fileResult) (files int, failed int, rowsTotal int, rowsOK int, rowsErr int) {
	files = len(results)
	for _, r := range results {
		if r.Status == runStatusFail {
			failed++
		}
		rowsTotal += r.RowsTotal
		rowsOK += r.RowsOK
		rowsErr += r.RowsError
	}
	return files, failed, rowsTotal, rowsOK, rowsErr
}

func collectFiles(root, ext string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ext) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func groupFilesByDate(files []string, inputRoot string) ([]string, map[string][]string) {
	grouped := make(map[string][]string, 2048)
	for _, path := range files {
		rel := path
		if r, err := filepath.Rel(inputRoot, path); err == nil {
			rel = filepath.ToSlash(r)
		}
		parts := strings.SplitN(rel, "/", 2)
		dateKey := parts[0]
		grouped[dateKey] = append(grouped[dateKey], path)
	}
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
		sort.Strings(grouped[k])
	}
	sort.Strings(keys)
	return keys, grouped
}

func ingestParsedGroup(ctx context.Context, pool *pgxpool.Pool, group parsedGroup, batchSize int) ([]fileResult, error) {
	results := make([]fileResult, len(group.results))
	copy(results, group.results)
	if len(group.parsed) == 0 {
		return results, group.parseErr
	}

	var lastErr error
	for attempt := 1; attempt <= maxFileTxnRetry; attempt++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return markGroupFailed(results), fmt.Errorf("begin tx: %w", err)
		}

		processErr := ingestParsedFilesTx(ctx, tx, group.parsed, batchSize, time.Now())
		if processErr != nil {
			_ = tx.Rollback(ctx)
			lastErr = processErr
			if isRetriableTxnErr(processErr) && attempt < maxFileTxnRetry {
				time.Sleep(time.Duration(attempt) * 150 * time.Millisecond)
				continue
			}
			return markGroupFailed(results), processErr
		}
		if err := tx.Commit(ctx); err != nil {
			lastErr = err
			if isRetriableTxnErr(err) && attempt < maxFileTxnRetry {
				time.Sleep(time.Duration(attempt) * 150 * time.Millisecond)
				continue
			}
			return markGroupFailed(results), err
		}
		for i := range results {
			if results[i].Status == runStatusFail {
				continue
			}
			if results[i].RowsError > 0 {
				results[i].Status = "partial_success"
			} else {
				results[i].Status = runStatusOK
			}
		}
		return results, group.parseErr
	}

	return markGroupFailed(results), lastErr
}

func parseFiles(files []string, inputRoot string, now time.Time) ([]parsedFile, error) {
	out := make([]parsedFile, 0, len(files))
	var firstErr error
	for _, absPath := range files {
		pf := parsedFile{
			result: fileResult{Status: runStatusOK},
		}
		relPath := absPath
		if r, err := filepath.Rel(inputRoot, absPath); err == nil {
			relPath = filepath.ToSlash(r)
		}
		pf.result.RelativePath = relPath

		raw, err := os.ReadFile(absPath)
		if err != nil {
			pf.result.Status = runStatusFail
			if firstErr == nil {
				firstErr = fmt.Errorf("read file %s: %w", relPath, err)
			}
			out = append(out, pf)
			continue
		}
		sum := sha256.Sum256(raw)
		pf.result.FileHash = hex.EncodeToString(sum[:])

		reader := csv.NewReader(strings.NewReader(string(raw)))
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true

		header, err := reader.Read()
		if err != nil {
			pf.result.Status = runStatusFail
			if firstErr == nil {
				firstErr = fmt.Errorf("read header %s: %w", relPath, err)
			}
			out = append(out, pf)
			continue
		}
		if len(header) == 0 {
			pf.result.Status = runStatusFail
			if firstErr == nil {
				firstErr = fmt.Errorf("empty header: %s", relPath)
			}
			out = append(out, pf)
			continue
		}
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
		idx := buildHeaderIndex(header)
		if err := validateRequiredColumns(idx); err != nil {
			pf.result.Status = runStatusFail
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", relPath, err)
			}
			out = append(out, pf)
			continue
		}

		rowNo := 1
		for {
			row, rerr := reader.Read()
			if errors.Is(rerr, io.EOF) {
				break
			}
			if rerr != nil {
				pf.result.Status = runStatusFail
				if firstErr == nil {
					firstErr = fmt.Errorf("csv read row %d (%s): %w", rowNo+1, relPath, rerr)
				}
				break
			}
			rowNo++
			pf.result.RowsTotal++

			pr, perr := parseRow(idx, row, rowNo, relPath, now)
			if perr != nil {
				pf.result.RowsError++
				continue
			}
			pf.rows = append(pf.rows, pr)
			pf.result.RowsOK++
		}
		out = append(out, pf)
	}
	return out, firstErr
}

func ingestParsedFilesTx(ctx context.Context, tx pgx.Tx, parsed []parsedFile, batchSize int, now time.Time) error {
	if err := ensureFactStageTable(ctx, tx); err != nil {
		return err
	}
	cache := &dimCache{
		market: make(map[string]uint, 64),
		item:   make(map[string]uint, 256),
		origin: make(map[string]uint, 256),
		grade:  make(map[string]uint, 128),
	}
	if err := preloadDimCaches(ctx, tx, parsed, cache, now); err != nil {
		return err
	}
	var batch []model.FactPricesDaily
	for _, pf := range parsed {
		if pf.result.Status == runStatusFail {
			continue
		}
		for _, pr := range pf.rows {
			marketID, ok := cache.market[pr.MarketCode]
			if !ok {
				return fmt.Errorf("market id not found for code=%s", pr.MarketCode)
			}
			itemID, ok := cache.item[pr.ItemCode]
			if !ok {
				return fmt.Errorf("item id not found for code=%s", pr.ItemCode)
			}
			originID, ok := cache.origin[pr.OriginCode]
			if !ok {
				return fmt.Errorf("origin id not found for code=%s", pr.OriginCode)
			}
			gradeKey := gradeCompositeKey(pr.Grade, pr.Class, pr.ProductName, pr.UnitWeight)
			gradeID, ok := cache.grade[gradeKey]
			if !ok {
				return fmt.Errorf("grade id not found for key=%s", gradeKey)
			}
			batch = append(batch, model.FactPricesDaily{
				TradeDate:    pr.TradeDate,
				WeekdayJA:    pr.WeekdayJA,
				MarketID:     marketID,
				ItemID:       itemID,
				OriginID:     originID,
				GradeID:      gradeID,
				ItemTotalTon: pr.ItemTotalTon,
				ArrivalTon:   pr.ArrivalTon,
				PriceHighYen: pr.PriceHighYen,
				PriceMidYen:  pr.PriceMidYen,
				PriceLowYen:  pr.PriceLowYen,
				TrendLabel:   pr.TrendLabel,
				SourceFile:   pr.SourceFile,
				SourceRowNo:  pr.SourceRowNo,
				IngestedAt:   pr.IngestedAt,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
			if len(batch) >= batchSize {
				if err := copyFactsToStage(ctx, tx, batch); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}
	}
	if len(batch) > 0 {
		if err := copyFactsToStage(ctx, tx, batch); err != nil {
			return err
		}
	}
	return mergeFactsFromStage(ctx, tx)
}

func markGroupFailed(results []fileResult) []fileResult {
	for i := range results {
		results[i].Status = runStatusFail
	}
	return results
}

type gradeDimValue struct {
	Grade       string
	Class       string
	ProductName string
	UnitWeight  string
}

func gradeCompositeKey(grade, class, productName, unitWeight string) string {
	return strings.Join([]string{grade, class, productName, unitWeight}, "|")
}

func preloadDimCaches(ctx context.Context, tx pgx.Tx, parsed []parsedFile, cache *dimCache, now time.Time) error {
	marketMap := make(map[string]string, 128)
	itemMap := make(map[string]string, 512)
	originMap := make(map[string]string, 512)
	gradeMap := make(map[string]gradeDimValue, 512)

	for _, pf := range parsed {
		if pf.result.Status == runStatusFail {
			continue
		}
		for _, pr := range pf.rows {
			marketMap[pr.MarketCode] = pr.MarketName
			itemMap[pr.ItemCode] = pr.ItemName
			originMap[pr.OriginCode] = pr.OriginName
			key := gradeCompositeKey(pr.Grade, pr.Class, pr.ProductName, pr.UnitWeight)
			gradeMap[key] = gradeDimValue{
				Grade:       pr.Grade,
				Class:       pr.Class,
				ProductName: pr.ProductName,
				UnitWeight:  pr.UnitWeight,
			}
		}
	}

	if err := upsertAndLoadMarketDims(ctx, tx, marketMap, cache, now); err != nil {
		return err
	}
	if err := upsertAndLoadItemDims(ctx, tx, itemMap, cache, now); err != nil {
		return err
	}
	if err := upsertAndLoadOriginDims(ctx, tx, originMap, cache, now); err != nil {
		return err
	}
	if err := upsertAndLoadGradeDims(ctx, tx, gradeMap, cache, now); err != nil {
		return err
	}
	return nil
}

func upsertAndLoadMarketDims(ctx context.Context, tx pgx.Tx, values map[string]string, cache *dimCache, now time.Time) error {
	if len(values) == 0 {
		return nil
	}
	codes := make([]string, 0, len(values))
	names := make([]string, 0, len(values))
	for code, name := range values {
		codes = append(codes, code)
		names = append(names, name)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO dim_market (market_code, market_name, created_at, updated_at)
SELECT code, name, $3, $3
FROM unnest($1::text[], $2::text[]) AS t(code, name)
ON CONFLICT (market_code)
DO UPDATE SET market_name = EXCLUDED.market_name, updated_at = EXCLUDED.updated_at
`, codes, names, now)
	if err != nil {
		return fmt.Errorf("bulk market upsert failed: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT id, market_code FROM dim_market WHERE market_code = ANY($1::text[])`, codes)
	if err != nil {
		return fmt.Errorf("load market ids failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return err
		}
		cache.market[code] = uint(id)
	}
	return rows.Err()
}

func upsertAndLoadItemDims(ctx context.Context, tx pgx.Tx, values map[string]string, cache *dimCache, now time.Time) error {
	if len(values) == 0 {
		return nil
	}
	codes := make([]string, 0, len(values))
	names := make([]string, 0, len(values))
	for code, name := range values {
		codes = append(codes, code)
		names = append(names, name)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO dim_item (item_code, item_name, created_at, updated_at)
SELECT code, name, $3, $3
FROM unnest($1::text[], $2::text[]) AS t(code, name)
ON CONFLICT (item_code)
DO UPDATE SET item_name = EXCLUDED.item_name, updated_at = EXCLUDED.updated_at
`, codes, names, now)
	if err != nil {
		return fmt.Errorf("bulk item upsert failed: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT id, item_code FROM dim_item WHERE item_code = ANY($1::text[])`, codes)
	if err != nil {
		return fmt.Errorf("load item ids failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return err
		}
		cache.item[code] = uint(id)
	}
	return rows.Err()
}

func upsertAndLoadOriginDims(ctx context.Context, tx pgx.Tx, values map[string]string, cache *dimCache, now time.Time) error {
	if len(values) == 0 {
		return nil
	}
	codes := make([]string, 0, len(values))
	names := make([]string, 0, len(values))
	for code, name := range values {
		codes = append(codes, code)
		names = append(names, name)
	}
	_, err := tx.Exec(ctx, `
INSERT INTO dim_origin (origin_code, origin_name, created_at, updated_at)
SELECT code, name, $3, $3
FROM unnest($1::text[], $2::text[]) AS t(code, name)
ON CONFLICT (origin_code)
DO UPDATE SET origin_name = EXCLUDED.origin_name, updated_at = EXCLUDED.updated_at
`, codes, names, now)
	if err != nil {
		return fmt.Errorf("bulk origin upsert failed: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT id, origin_code FROM dim_origin WHERE origin_code = ANY($1::text[])`, codes)
	if err != nil {
		return fmt.Errorf("load origin ids failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return err
		}
		cache.origin[code] = uint(id)
	}
	return rows.Err()
}

func upsertAndLoadGradeDims(ctx context.Context, tx pgx.Tx, values map[string]gradeDimValue, cache *dimCache, now time.Time) error {
	if len(values) == 0 {
		return nil
	}
	grades := make([]string, 0, len(values))
	classes := make([]string, 0, len(values))
	productNames := make([]string, 0, len(values))
	unitWeights := make([]string, 0, len(values))
	for _, v := range values {
		grades = append(grades, v.Grade)
		classes = append(classes, v.Class)
		productNames = append(productNames, v.ProductName)
		unitWeights = append(unitWeights, v.UnitWeight)
	}

	_, err := tx.Exec(ctx, `
INSERT INTO dim_grade (grade, class, product_name, unit_weight, created_at, updated_at)
SELECT grade, class, product_name, unit_weight, $5, $5
FROM unnest($1::text[], $2::text[], $3::text[], $4::text[]) AS t(grade, class, product_name, unit_weight)
ON CONFLICT (grade, class, product_name, unit_weight)
DO UPDATE SET updated_at = EXCLUDED.updated_at
`, grades, classes, productNames, unitWeights, now)
	if err != nil {
		return fmt.Errorf("bulk grade upsert failed: %w", err)
	}
	rows, err := tx.Query(ctx, `
SELECT d.id, d.grade, d.class, d.product_name, d.unit_weight
FROM dim_grade d
JOIN (
	SELECT grade, class, product_name, unit_weight
	FROM unnest($1::text[], $2::text[], $3::text[], $4::text[]) AS t(grade, class, product_name, unit_weight)
) k
ON d.grade = k.grade
AND d.class = k.class
AND d.product_name = k.product_name
AND d.unit_weight = k.unit_weight
`, grades, classes, productNames, unitWeights)
	if err != nil {
		return fmt.Errorf("load grade ids failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var grade, class, productName, unitWeight string
		if err := rows.Scan(&id, &grade, &class, &productName, &unitWeight); err != nil {
			return err
		}
		cache.grade[gradeCompositeKey(grade, class, productName, unitWeight)] = uint(id)
	}
	return rows.Err()
}

func buildHeaderIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.TrimSpace(h)
		if key != "" {
			idx[key] = i
		}
	}
	return idx
}

func validateRequiredColumns(idx map[string]int) error {
	required := []string{
		"年", "月", "日", "曜日",
		"市場名", "市場コード",
		"品目名", "品目コード",
		"産地名", "産地コード",
		"品目計", "入荷量",
		"高値", "中値", "安値",
		"等級", "階級", "品名", "量目", "動向",
	}
	for _, k := range required {
		if _, ok := idx[k]; !ok {
			return fmt.Errorf("missing required column: %s", k)
		}
	}
	return nil
}

func parseRow(idx map[string]int, row []string, rowNo int, sourceFile string, now time.Time) (parsedRow, error) {
	get := func(name string) string {
		i, ok := idx[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	year, err := parseRequiredInt(get("年"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 年: %w", rowNo, err)
	}
	month, err := parseRequiredInt(get("月"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 月: %w", rowNo, err)
	}
	day, err := parseRequiredInt(get("日"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 日: %w", rowNo, err)
	}
	tradeDate, err := time.Parse("2006-1-2", fmt.Sprintf("%d-%d-%d", year, month, day))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid date: %w", rowNo, err)
	}

	marketCode := normalizeCode(get("市場コード"))
	itemCode := normalizeCode(get("品目コード"))
	originCode := normalizeCode(get("産地コード"))
	if marketCode == "" || itemCode == "" {
		return parsedRow{}, fmt.Errorf("row=%d missing required codes", rowNo)
	}
	if originCode == "" {
		originCode = unknownOriginCode
	}

	itemTotalTon, err := parseNullableFloat(get("品目計"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 品目計: %w", rowNo, err)
	}
	arrivalTon, err := parseNullableFloat(get("入荷量"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 入荷量: %w", rowNo, err)
	}
	priceHigh, err := parseNullableInt(get("高値"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 高値: %w", rowNo, err)
	}
	priceMid, err := parseNullableInt(get("中値"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 中値: %w", rowNo, err)
	}
	priceLow, err := parseNullableInt(get("安値"))
	if err != nil {
		return parsedRow{}, fmt.Errorf("row=%d invalid 安値: %w", rowNo, err)
	}
	if itemTotalTon != nil && *itemTotalTon < 0 {
		return parsedRow{}, fmt.Errorf("row=%d invalid 品目計: negative", rowNo)
	}
	if arrivalTon != nil && *arrivalTon < 0 {
		return parsedRow{}, fmt.Errorf("row=%d invalid 入荷量: negative", rowNo)
	}
	if priceHigh != nil && *priceHigh < 0 {
		return parsedRow{}, fmt.Errorf("row=%d invalid 高値: negative", rowNo)
	}
	if priceMid != nil && *priceMid < 0 {
		return parsedRow{}, fmt.Errorf("row=%d invalid 中値: negative", rowNo)
	}
	if priceLow != nil && *priceLow < 0 {
		return parsedRow{}, fmt.Errorf("row=%d invalid 安値: negative", rowNo)
	}

	trend := normalizeOptional(get("動向"))
	var trendPtr *string
	if trend != "" {
		trendPtr = &trend
	}
	originName := normalizeName(get("産地名"))
	if originName == "" {
		originName = unknownOriginName
	}
	if normalizeName(get("市場名")) == "" || normalizeName(get("品目名")) == "" {
		return parsedRow{}, fmt.Errorf("row=%d missing required names", rowNo)
	}

	return parsedRow{
		TradeDate:    tradeDate,
		WeekdayJA:    normalizeOptional(get("曜日")),
		MarketCode:   marketCode,
		MarketName:   normalizeName(get("市場名")),
		ItemCode:     itemCode,
		ItemName:     normalizeName(get("品目名")),
		OriginCode:   originCode,
		OriginName:   originName,
		Grade:        normalizeOptional(get("等級")),
		Class:        normalizeOptional(get("階級")),
		ProductName:  normalizeOptional(get("品名")),
		UnitWeight:   normalizeOptional(get("量目")),
		ItemTotalTon: itemTotalTon,
		ArrivalTon:   arrivalTon,
		PriceHighYen: priceHigh,
		PriceMidYen:  priceMid,
		PriceLowYen:  priceLow,
		TrendLabel:   trendPtr,
		SourceFile:   sourceFile,
		SourceRowNo:  rowNo,
		IngestedAt:   now,
	}, nil
}

func parseRequiredInt(v string) (int, error) {
	v = normalizeOptional(v)
	if v == "" {
		return 0, errors.New("empty")
	}
	return strconv.Atoi(v)
}

func parseNullableFloat(v string) (*float64, error) {
	v = normalizeOptional(v)
	if v == "" || v == "-" {
		return nil, nil
	}
	v = strings.ReplaceAll(v, ",", "")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func parseNullableInt(v string) (*int, error) {
	v = normalizeOptional(v)
	if v == "" || v == "-" {
		return nil, nil
	}
	v = strings.ReplaceAll(v, ",", "")
	if n, err := strconv.Atoi(v); err == nil {
		return &n, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, err
	}
	n := int(f)
	return &n, nil
}

func normalizeCode(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "")
	return s
}

func normalizeOptional(s string) string {
	return strings.TrimSpace(s)
}

func normalizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "")
	return s
}

func buildPGXConnString(cfg config.Config) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s timezone=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
		cfg.Database.TimeZone,
	)
}

func ensureFactStageTable(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
CREATE TEMP TABLE IF NOT EXISTS fact_prices_daily_stage (
	trade_date date NOT NULL,
	weekday_ja varchar(16) NOT NULL,
	market_id bigint NOT NULL,
	item_id bigint NOT NULL,
	origin_id bigint NOT NULL,
	grade_id bigint NOT NULL,
	item_total_ton numeric(14,3),
	arrival_ton numeric(14,3),
	price_high_yen integer,
	price_mid_yen integer,
	price_low_yen integer,
	trend_label varchar(64),
	source_file varchar(1024) NOT NULL,
	source_row_no integer NOT NULL,
	ingested_at timestamptz NOT NULL,
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
) ON COMMIT DROP`)
	if err != nil {
		return fmt.Errorf("create temp stage table failed: %w", err)
	}
	return nil
}

func copyFactsToStage(ctx context.Context, tx pgx.Tx, rows []model.FactPricesDaily) error {
	if len(rows) == 0 {
		return nil
	}
	copyRows := make([][]any, 0, len(rows))
	for _, r := range rows {
		copyRows = append(copyRows, []any{
			r.TradeDate,
			r.WeekdayJA,
			int64(r.MarketID),
			int64(r.ItemID),
			int64(r.OriginID),
			int64(r.GradeID),
			r.ItemTotalTon,
			r.ArrivalTon,
			r.PriceHighYen,
			r.PriceMidYen,
			r.PriceLowYen,
			r.TrendLabel,
			r.SourceFile,
			r.SourceRowNo,
			r.IngestedAt,
			r.CreatedAt,
			r.UpdatedAt,
		})
	}
	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"fact_prices_daily_stage"},
		[]string{
			"trade_date",
			"weekday_ja",
			"market_id",
			"item_id",
			"origin_id",
			"grade_id",
			"item_total_ton",
			"arrival_ton",
			"price_high_yen",
			"price_mid_yen",
			"price_low_yen",
			"trend_label",
			"source_file",
			"source_row_no",
			"ingested_at",
			"created_at",
			"updated_at",
		},
		pgx.CopyFromRows(copyRows),
	)
	if err != nil {
		return fmt.Errorf("copy facts to stage failed: %w", err)
	}
	return nil
}

func mergeFactsFromStage(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
INSERT INTO fact_prices_daily (
	trade_date, weekday_ja, market_id, item_id, origin_id, grade_id,
	item_total_ton, arrival_ton, price_high_yen, price_mid_yen, price_low_yen,
	trend_label, source_file, source_row_no, ingested_at, created_at, updated_at
)
SELECT
	trade_date, weekday_ja, market_id, item_id, origin_id, grade_id,
	item_total_ton, arrival_ton, price_high_yen, price_mid_yen, price_low_yen,
	trend_label, source_file, source_row_no, ingested_at, created_at, updated_at
FROM fact_prices_daily_stage
ON CONFLICT (source_file, source_row_no)
DO UPDATE SET
	trade_date = EXCLUDED.trade_date,
	weekday_ja = EXCLUDED.weekday_ja,
	market_id = EXCLUDED.market_id,
	item_id = EXCLUDED.item_id,
	origin_id = EXCLUDED.origin_id,
	grade_id = EXCLUDED.grade_id,
	item_total_ton = EXCLUDED.item_total_ton,
	arrival_ton = EXCLUDED.arrival_ton,
	price_high_yen = EXCLUDED.price_high_yen,
	price_mid_yen = EXCLUDED.price_mid_yen,
	price_low_yen = EXCLUDED.price_low_yen,
	trend_label = EXCLUDED.trend_label,
	ingested_at = EXCLUDED.ingested_at,
	updated_at = EXCLUDED.updated_at`)
	if err != nil {
		return fmt.Errorf("merge facts from stage failed: %w", err)
	}
	return nil
}

func getOrCreateMarketPGX(ctx context.Context, tx pgx.Tx, cache *dimCache, code, name string, now time.Time) (uint, error) {
	if id, ok := cache.market[code]; ok {
		return id, nil
	}
	var id uint
	err := tx.QueryRow(ctx, `
INSERT INTO dim_market (market_code, market_name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (market_code)
DO UPDATE SET market_name = EXCLUDED.market_name, updated_at = EXCLUDED.updated_at
RETURNING id
`, code, name, now).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("market upsert failed (code=%s): %w", code, err)
	}
	cache.market[code] = id
	return id, nil
}

func getOrCreateItemPGX(ctx context.Context, tx pgx.Tx, cache *dimCache, code, name string, now time.Time) (uint, error) {
	if id, ok := cache.item[code]; ok {
		return id, nil
	}
	var id uint
	err := tx.QueryRow(ctx, `
INSERT INTO dim_item (item_code, item_name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (item_code)
DO UPDATE SET item_name = EXCLUDED.item_name, updated_at = EXCLUDED.updated_at
RETURNING id
`, code, name, now).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("item upsert failed (code=%s): %w", code, err)
	}
	cache.item[code] = id
	return id, nil
}

func getOrCreateOriginPGX(ctx context.Context, tx pgx.Tx, cache *dimCache, code, name string, now time.Time) (uint, error) {
	if id, ok := cache.origin[code]; ok {
		return id, nil
	}
	var id uint
	err := tx.QueryRow(ctx, `
INSERT INTO dim_origin (origin_code, origin_name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (origin_code)
DO UPDATE SET origin_name = EXCLUDED.origin_name, updated_at = EXCLUDED.updated_at
RETURNING id
`, code, name, now).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("origin upsert failed (code=%s): %w", code, err)
	}
	cache.origin[code] = id
	return id, nil
}

func getOrCreateGradePGX(ctx context.Context, tx pgx.Tx, cache *dimCache, grade, class, productName, unitWeight string, now time.Time) (uint, error) {
	key := strings.Join([]string{grade, class, productName, unitWeight}, "|")
	if id, ok := cache.grade[key]; ok {
		return id, nil
	}
	var id uint
	err := tx.QueryRow(ctx, `
INSERT INTO dim_grade (grade, class, product_name, unit_weight, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (grade, class, product_name, unit_weight)
DO UPDATE SET updated_at = EXCLUDED.updated_at
RETURNING id
`, grade, class, productName, unitWeight, now).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("grade upsert failed: %w", err)
	}
	cache.grade[key] = id
	return id, nil
}

func isRetriableTxnErr(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40P01" || pgErr.Code == "25P02"
	}
	return false
}
