// Package testutil provides shared mock adapters for use in tests.
// All mocks implement the relevant port interface and are safe to use
// with zero configuration — sensible defaults are returned unless a
// specific Fn field is set.
package testutil

import (
	"context"
	"time"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
)

// --- MockAI ---

// MockAI implements ports.AIPort for testing.
// Override individual Fn fields to inject specific behaviour.
type MockAI struct {
	EmbedFn           func(ctx context.Context, text string) ([]float32, error)
	DetectTypeFn      func(ctx context.Context, content string) (domain.MemoryType, error)
	ExtractTagsFn     func(ctx context.Context, content, forLabel string) ([]string, error)
	AnswerFn          func(ctx context.Context, question string, memories []*domain.Memory) (string, error)
	ExpandQueryFn     func(ctx context.Context, question string) (string, error)
	RerankFn          func(ctx context.Context, query string, candidates []*domain.Memory) ([]*domain.Memory, error)
	ExtractEntitiesFn func(ctx context.Context, content string) ([]domain.Entity, error)
}

var _ ports.AIPort = (*MockAI)(nil)

func (m *MockAI) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedFn != nil {
		return m.EmbedFn(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockAI) DetectType(ctx context.Context, content string) (domain.MemoryType, error) {
	if m.DetectTypeFn != nil {
		return m.DetectTypeFn(ctx, content)
	}
	return domain.MemoryTypeNote, nil
}

func (m *MockAI) ExtractTags(ctx context.Context, content, forLabel string) ([]string, error) {
	if m.ExtractTagsFn != nil {
		return m.ExtractTagsFn(ctx, content, forLabel)
	}
	return []string{"test"}, nil
}

func (m *MockAI) Answer(ctx context.Context, question string, memories []*domain.Memory) (string, error) {
	if m.AnswerFn != nil {
		return m.AnswerFn(ctx, question, memories)
	}
	return "mock answer", nil
}

// ExpandQuery returns the question as-is by default (no-op HyDE in tests).
func (m *MockAI) ExpandQuery(ctx context.Context, question string) (string, error) {
	if m.ExpandQueryFn != nil {
		return m.ExpandQueryFn(ctx, question)
	}
	return question, nil
}

// Rerank returns candidates in original order by default (no-op in tests).
func (m *MockAI) Rerank(ctx context.Context, query string, candidates []*domain.Memory) ([]*domain.Memory, error) {
	if m.RerankFn != nil {
		return m.RerankFn(ctx, query, candidates)
	}
	return candidates, nil
}

// ExtractEntities returns an empty slice by default.
func (m *MockAI) ExtractEntities(ctx context.Context, content string) ([]domain.Entity, error) {
	if m.ExtractEntitiesFn != nil {
		return m.ExtractEntitiesFn(ctx, content)
	}
	return []domain.Entity{}, nil
}

// --- MockTimeParser ---

// MockTimeParser implements ports.TimeParserPort for testing.
// By default, Parse returns (from + 30 minutes).
type MockTimeParser struct {
	ParseFn func(expr string, from time.Time) (*time.Time, error)
}

var _ ports.TimeParserPort = (*MockTimeParser)(nil)

func (m *MockTimeParser) Parse(expr string, from time.Time) (*time.Time, error) {
	if m.ParseFn != nil {
		return m.ParseFn(expr, from)
	}
	t := from.Add(30 * time.Minute)
	return &t, nil
}

// --- MockNotifier ---

// MockNotifier implements ports.NotifierPort for testing.
// It records all fired memories so tests can assert on them.
type MockNotifier struct {
	Fired    []*domain.Memory
	NotifyFn func(ctx context.Context, m *domain.Memory) error
}

var _ ports.NotifierPort = (*MockNotifier)(nil)

func (m *MockNotifier) Notify(ctx context.Context, mem *domain.Memory) error {
	if m.NotifyFn != nil {
		return m.NotifyFn(ctx, mem)
	}
	m.Fired = append(m.Fired, mem)
	return nil
}
