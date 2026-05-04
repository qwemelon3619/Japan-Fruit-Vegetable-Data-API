package grpc

import (
	"context"
	"time"

	japanapiv1 "japan_data_project/proto/japanapi/v1"
	v1svc "japan_data_project/internal/app/api/handler/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ingestionService implements the IngestionService gRPC server.
type ingestionService struct {
	japanapiv1.UnimplementedIngestionServiceServer
	svc *v1svc.Service
}

// ListIngestionRuns returns a paginated list of ingestion runs.
func (s *ingestionService) ListIngestionRuns(ctx context.Context, req *japanapiv1.ListIngestionRunsRequest) (*japanapiv1.ListIngestionRunsResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.GetOffset())

	result, err := s.svc.ListIngestionRuns(ctx, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list ingestion runs failed: %v", err)
	}

	pbRows := make([]*japanapiv1.IngestionRunRow, 0, len(result.Rows))
	for _, r := range result.Rows {
		pbRow := &japanapiv1.IngestionRunRow{
			Id:      uint32(r.ID),
			RunType: r.RunType,
			Status:  r.Status,
		}
		pbRow.StartedAt = r.StartedAt.Format(time.RFC3339)
		if r.FinishedAt != nil {
			formatted := r.FinishedAt.Format(time.RFC3339)
			pbRow.FinishedAt = &formatted
		}
		if r.ErrorMessage != nil {
			pbRow.ErrorMessage = r.ErrorMessage
		}
		pbRows = append(pbRows, pbRow)
	}

	return &japanapiv1.ListIngestionRunsResponse{
		Data:   pbRows,
		Limit:  int32(result.Limit),
		Offset: int32(result.Offset),
		Total:  result.Total,
	}, nil
}

// ListIngestionFiles returns a paginated list of ingestion files, optionally filtered by run_id.
func (s *ingestionService) ListIngestionFiles(ctx context.Context, req *japanapiv1.ListIngestionFilesRequest) (*japanapiv1.ListIngestionFilesResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.GetOffset())

	var runID *uint
	if req.RunId != nil {
		id := uint(*req.RunId)
		runID = &id
	}

	result, err := s.svc.ListIngestionFiles(ctx, runID, limit, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list ingestion files failed: %v", err)
	}

	pbRows := make([]*japanapiv1.IngestionFileRow, 0, len(result.Rows))
	for _, r := range result.Rows {
		pbRows = append(pbRows, &japanapiv1.IngestionFileRow{
			Id:        uint32(r.ID),
			RunId:     uint32(r.RunID),
			FilePath:  r.FilePath,
			FileHash:  r.FileHash,
			RowsTotal: int32(r.RowsTotal),
			RowsOk:    int32(r.RowsOK),
			RowsError: int32(r.RowsError),
			Status:    r.Status,
		})
	}

	return &japanapiv1.ListIngestionFilesResponse{
		Data:   pbRows,
		Limit:  int32(result.Limit),
		Offset: int32(result.Offset),
		Total:  result.Total,
	}, nil
}
