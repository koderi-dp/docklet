package services

import (
	"context"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// OperationService exposes the background-operation manager to the frontend.
type OperationService struct {
	Ops *ops.Manager
	// History lists finished operations from previous sessions (metadata only). Optional.
	History func(ctx context.Context, limit int) ([]model.Operation, error)
}

// PastOperations returns persisted finished operations, newest first.
func (s *OperationService) PastOperations(ctx context.Context, limit int) ([]model.Operation, error) {
	if s.History == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.History(ctx, limit)
}

func (s *OperationService) List(ctx context.Context) []model.Operation { return s.Ops.List() }

func (s *OperationService) Logs(ctx context.Context, id string) ([]string, error) {
	return s.Ops.Logs(id)
}

// Cancel stops client-side work; server-side cancellation depends on the Docker API involved.
func (s *OperationService) Cancel(ctx context.Context, id string) error { return s.Ops.Cancel(id) }

func (s *OperationService) ClearFinished(ctx context.Context) { s.Ops.ClearFinished() }
