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
	EmbedFn  func(ctx context.Context, text string) ([]float32, error)
	AnswerFn func(ctx context.Context, question string, memories []*domain.Memory) (string, error)
}

var _ ports.AIPort = (*MockAI)(nil)

func (m *MockAI) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedFn != nil {
		return m.EmbedFn(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *MockAI) Answer(ctx context.Context, question string, memories []*domain.Memory) (string, error) {
	if m.AnswerFn != nil {
		return m.AnswerFn(ctx, question, memories)
	}
	return "mock answer", nil
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
