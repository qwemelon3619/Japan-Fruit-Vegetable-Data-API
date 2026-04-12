package downloader

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"japan_data_project/internal/platform/config"
	"japan_data_project/internal/platform/logger"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const (
	dateListURL  = "https://www.seisen.maff.go.jp/seisen/bs04b040md001/BS04B040UC010SC999-Evt001.do"
	dateClickURL = "https://www.seisen.maff.go.jp/seisen/bs04b040md001/BS04B040UC010SC001-Evt001.do"
	csvURL       = "https://www.seisen.maff.go.jp/seisen/bs04b040md001/BS04B040UC010SC001-Evt004.do"
)

type options struct {
	date         string
	from         string
	to           string
	outDir       string
	marketFilter string
	saveRaw      bool
	saveUTF8     bool
	utf8BOM      bool
	skipNoData   bool
	timeout      time.Duration
	waitPerDate  time.Duration
}

type marketRow struct {
	No     string
	Market string
	CSVIDs []string
}

var (
	reDateInput  = regexp.MustCompile(`name="s006\.dataDate"\s+value="(\d{8})"`)
	reDateHeader = regexp.MustCompile(`(?s)<h3 class="h3_title"[^>]*>\s*([^<]+?)\s*</h3>`)
	reRowBlock   = regexp.MustCompile(`(?s)<tr>\s*<td class="kubun_td3_norb_left">\s*([^<]+?)\s*</td>\s*<td class="kubun_td3_norb_left">\s*([^<]+?)\s*</td>(.*?)</tr>`)
	reCSVID      = regexp.MustCompile(`chohyoSubmit\('form003','([^']+)'\)`)
	reDispName   = regexp.MustCompile(`filename\*?=(?:UTF-8''|"?)([^";\r\n]+)`)
)

func Run(args []string) error {
	cfg := config.Load()
	log := logger.New(cfg)

	opts, err := parseFlags(args)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: opts.timeout}
	dates, err := resolveDates(client, opts)
	if err != nil {
		return fmt.Errorf("failed to resolve dates: %w", err)
	}
	if len(dates) == 0 {
		return errors.New("no target dates")
	}

	log.Info("downloader started", "dates", len(dates), "out", opts.outDir)

	totalFiles := 0
	for i, d := range dates {
		rows, dateLabel, err := fetchMarketRows(client, d)
		if err != nil {
			if opts.skipNoData {
				log.Warn("skip date: failed to fetch rows", "date", d, "error", err)
				continue
			}
			return fmt.Errorf("failed to fetch date rows (date=%s): %w", d, err)
		}

		selectedRows := filterRows(rows, opts.marketFilter)
		if len(selectedRows) == 0 {
			log.Warn("no rows after market filter", "date", d, "market_filter", opts.marketFilter)
			continue
		}

		log.Info("processing date", "date", d, "label", dateLabel, "markets", len(selectedRows))

		for _, r := range selectedRows {
			for i, id := range r.CSVIDs {
				category := categoryName(i)
				baseName := fmt.Sprintf("%s_%s_%s", r.No, sanitize(r.Market), category)

				rawFilename, rawBytes, err := downloadRawCSV(client, id, baseName)
				if err != nil {
					log.Warn("csv download failed", "date", d, "market", r.Market, "category", category, "error", err)
					continue
				}

				if opts.saveRaw {
					rawPath := filepath.Join(opts.outDir, "raw", d, rawFilename)
					if err := writeFile(rawPath, rawBytes); err != nil {
						log.Warn("raw save failed", "path", rawPath, "error", err)
						continue
					}
				}

				if opts.saveUTF8 {
					utf8Path := filepath.Join(opts.outDir, "utf8", d, rawFilename)
					utf8Bytes, err := toUTF8(rawBytes, opts.utf8BOM)
					if err != nil {
						log.Warn("utf8 convert failed", "file", rawFilename, "error", err)
						continue
					}
					if err := writeFile(utf8Path, utf8Bytes); err != nil {
						log.Warn("utf8 save failed", "path", utf8Path, "error", err)
						continue
					}
				}

				totalFiles++
				log.Info("saved csv", "date", d, "market", r.Market, "category", category, "file", rawFilename)
			}
		}

		if i < len(dates)-1 && opts.waitPerDate > 0 {
			log.Info("waiting before next date", "wait", opts.waitPerDate.String(), "next_index", i+2, "total_dates", len(dates))
			time.Sleep(opts.waitPerDate)
		}
	}

	log.Info("downloader finished", "total_files", totalFiles)
	return nil
}

func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.StringVar(&o.date, "date", "", "single date YYYYMMDD")
	fs.StringVar(&o.from, "from", "", "start date YYYYMMDD (inclusive)")
	fs.StringVar(&o.to, "to", "", "end date YYYYMMDD (inclusive)")
	fs.StringVar(&o.outDir, "out", "data/data_downloads", "output directory")
	fs.StringVar(&o.marketFilter, "market", "", "market name contains filter")
	fs.BoolVar(&o.saveRaw, "save-raw", true, "save original raw csv bytes")
	fs.BoolVar(&o.saveUTF8, "save-utf8", true, "save utf8 converted csv")
	fs.BoolVar(&o.utf8BOM, "utf8-bom", true, "add UTF-8 BOM when saving utf8")
	fs.BoolVar(&o.skipNoData, "skip-no-data", true, "skip date when no data exists")
	fs.DurationVar(&o.timeout, "timeout", 30*time.Second, "http timeout")
	fs.DurationVar(&o.waitPerDate, "wait-per-date", 10*time.Second, "wait duration between dates (e.g. 10s, 1m)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	return o, nil
}

func resolveDates(client *http.Client, o options) ([]string, error) {
	if o.date != "" {
		return []string{o.date}, nil
	}

	if o.from != "" && o.to != "" {
		from, err := time.Parse("20060102", o.from)
		if err != nil {
			return nil, fmt.Errorf("invalid -from: %w", err)
		}
		to, err := time.Parse("20060102", o.to)
		if err != nil {
			return nil, fmt.Errorf("invalid -to: %w", err)
		}
		if from.After(to) {
			return nil, errors.New("-from must be <= -to")
		}
		var out []string
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			out = append(out, d.Format("20060102"))
		}
		return out, nil
	}

	body, _, err := request(client, http.MethodGet, dateListURL, "", nil)
	if err != nil {
		return nil, err
	}
	matches := reDateInput.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, errors.New("could not parse available dates")
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out, nil
}

func fetchMarketRows(client *http.Client, date string) ([]marketRow, string, error) {
	form := url.Values{}
	form.Set("s006.dataDate", date)
	body, _, err := request(client, http.MethodPost, dateClickURL, form.Encode(), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		return nil, "", err
	}

	dateLabel := date
	if m := reDateHeader.FindStringSubmatch(body); len(m) > 1 {
		dateLabel = cleanText(m[1])
	}

	blocks := reRowBlock.FindAllStringSubmatch(body, -1)
	if len(blocks) == 0 {
		return nil, dateLabel, errors.New("no market table rows")
	}

	var rows []marketRow
	for _, b := range blocks {
		rowNo := cleanText(b[1])
		marketName := cleanText(b[2])
		csvMatches := reCSVID.FindAllStringSubmatch(b[3], -1)
		if marketName == "" || len(csvMatches) == 0 {
			continue
		}
		var ids []string
		for _, m := range csvMatches {
			ids = append(ids, m[1])
		}
		rows = append(rows, marketRow{No: rowNo, Market: marketName, CSVIDs: ids})
	}
	if len(rows) == 0 {
		return nil, dateLabel, errors.New("no csv rows parsed")
	}
	return rows, dateLabel, nil
}

func filterRows(rows []marketRow, marketFilter string) []marketRow {
	if strings.TrimSpace(marketFilter) == "" {
		return rows
	}
	filter := strings.ToLower(strings.TrimSpace(marketFilter))
	var out []marketRow
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Market), filter) {
			out = append(out, r)
		}
	}
	return out
}

func downloadRawCSV(client *http.Client, csvID string, baseName string) (string, []byte, error) {
	form := url.Values{}
	form.Set("s004.chohyoKanriNo", csvID)

	data, hdr, err := requestBytes(client, http.MethodPost, csvURL, form.Encode(), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		return "", nil, err
	}
	if len(data) == 0 {
		return "", nil, errors.New("empty csv response")
	}

	filename := resolveFilename(hdr.Get("Content-Disposition"), baseName)
	return filename, data, nil
}

func toUTF8(raw []byte, addBOM bool) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(raw), japanese.ShiftJIS.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if addBOM {
		decoded = append([]byte{0xEF, 0xBB, 0xBF}, decoded...)
	}
	return decoded, nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func resolveFilename(contentDisposition, fallbackBase string) string {
	name := ""
	if m := reDispName.FindStringSubmatch(contentDisposition); len(m) > 1 {
		name = strings.TrimSpace(m[1])
		name = strings.TrimPrefix(strings.ToLower(name), "utf-8''")
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}
	if name == "" {
		name = fallbackBase + ".csv"
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if ext == "" {
		ext = ".csv"
	}
	return sanitize(fallbackBase+"_"+base) + ext
}

func request(client *http.Client, method, u, body string, headers map[string]string) (string, http.Header, error) {
	bb, hdr, err := requestBytes(client, method, u, body, headers)
	if err != nil {
		return "", nil, err
	}
	return string(bb), hdr, nil
}

func requestBytes(client *http.Client, method, u, body string, headers map[string]string) ([]byte, http.Header, error) {
	var r io.Reader
	if body != "" {
		r = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; japan-data-downloader/1.0)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, fmt.Errorf("status=%d body=%q", resp.StatusCode, cleanText(string(data)))
	}
	return data, resp.Header, nil
}

func categoryName(i int) string {
	switch i {
	case 0:
		return "vegetable"
	case 1:
		return "fruit"
	default:
		return fmt.Sprintf("col%d", i+1)
	}
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(s)
	return strings.Join(strings.Fields(s), " ")
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_", " ", "_",
	)
	out := r.Replace(s)
	if out == "" {
		return "unknown"
	}
	return out
}
