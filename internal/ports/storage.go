package ports

import (
	"context"
	"time"

	"github.com/kunalsin9h/yaad/internal/domain"
)

// StoragePort defines all persistence operations.
// Implemented by: adapters/sqlite.Store
type StoragePort interface {
	Save(ctx context.Context, m *domain.Memory) error
	GetByID(ctx context.Context, id string) (*domain.Memory, error)
	List(ctx context.Context, filter domain.ListFilter) ([]*domain.Memory, error)
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context) (int64, error)
	FindSimilar(ctx context.Context, embedding []float32, topK int) ([]*domain.Memory, error)
	PendingReminders(ctx context.Context, before time.Time) ([]*domain.Memory, error)
	MarkReminded(ctx context.Context, id string) error
}
