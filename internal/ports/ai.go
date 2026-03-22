package ports

import (
	"context"

	"github.com/kunalsin9h/yaad/internal/domain"
)

// AIPort defines all intelligence operations.
// Implemented by: adapters/ollama.Client
type AIPort interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Answer(ctx context.Context, question string, memories []*domain.Memory) (string, error)
}
