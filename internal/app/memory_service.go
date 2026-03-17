package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
	"github.com/oklog/ulid/v2"
	"golang.org/x/sync/errgroup"
)

// AddRequest carries all parameters for MemoryService.Add.
type AddRequest struct {
	Content    string
	ForLabel   string
	RemindExpr string           // natural language, e.g. "in 30 minutes"
	TypeHint   domain.MemoryType // overrides AI detection when non-empty
	ExtraTags  []string          // user-provided tags merged with AI tags
}

// MemoryService orchestrates memory creation and retrieval.
type MemoryService struct {
	store ports.StoragePort
	ai    ports.AIPort
	timer ports.TimeParserPort
}

func NewMemoryService(store ports.StoragePort, ai ports.AIPort, timer ports.TimeParserPort) *MemoryService {
	return &MemoryService{store: store, ai: ai, timer: timer}
}

// Add creates and persists a new memory.
// AI enrichment (embedding, type detection, tag extraction) runs in parallel
// and is non-fatal — the memory is saved even if Ollama is unavailable.
func (s *MemoryService) Add(ctx context.Context, req AddRequest) (*domain.Memory, error) {
	now := time.Now()

	m := &domain.Memory{
		ID:        newID(),
		Content:   req.Content,
		ForLabel:  req.ForLabel,
		Type:      domain.MemoryTypeNote,
		Tags:      req.ExtraTags,
		CreatedAt: now,
	}

	// Capture system context automatically.
	if wd, err := os.Getwd(); err == nil {
		m.WorkingDir = wd
	}
	if h, err := os.Hostname(); err == nil {
		m.Hostname = h
	}

	// Parse reminder expression if provided.
	if req.RemindExpr != "" {
		t, err := s.timer.Parse(req.RemindExpr, now)
		if err != nil {
			return nil, fmt.Errorf("parse remind time: %w", err)
		}
		m.RemindAt = t
		if req.TypeHint == "" {
			req.TypeHint = domain.MemoryTypeReminder
		}
	}

	if req.TypeHint != "" {
		m.Type = req.TypeHint
	}

	// Run AI enrichment concurrently. Errors are warned but never block the save.
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		text := req.Content
		if req.ForLabel != "" {
			text += " " + req.ForLabel
		}
		emb, err := s.ai.Embed(gctx, text)
		if err != nil {
			if !errors.Is(err, domain.ErrOllamaUnavailable) {
				fmt.Fprintf(os.Stderr, "warn: embed: %v\n", err)
			}
			return nil
		}
		m.Embedding = emb
		return nil
	})

	if req.TypeHint == "" {
		g.Go(func() error {
			t, err := s.ai.DetectType(gctx, req.Content)
			if err != nil {
				return nil
			}
			m.Type = t
			return nil
		})
	}

	g.Go(func() error {
		tags, err := s.ai.ExtractTags(gctx, req.Content, req.ForLabel)
		if err != nil {
			return nil
		}
		m.Tags = append(m.Tags, tags...)
		return nil
	})

	_ = g.Wait() // all errors are non-fatal, handled inside goroutines

	if err := s.store.Save(ctx, m); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}
	return m, nil
}

// Ask answers a natural language question using semantic search + LLM.
func (s *MemoryService) Ask(ctx context.Context, question string) (string, error) {
	emb, err := s.ai.Embed(ctx, question)
	if err != nil {
		return "", fmt.Errorf("embed question: %w", err)
	}

	memories, err := s.store.FindSimilar(ctx, emb, 5)
	if err != nil {
		return "", fmt.Errorf("find similar: %w", err)
	}
	if len(memories) == 0 {
		return "No relevant memories found.", nil
	}

	return s.ai.Answer(ctx, question, memories)
}

func (s *MemoryService) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Memory, error) {
	return s.store.List(ctx, filter)
}

func (s *MemoryService) GetByID(ctx context.Context, id string) (*domain.Memory, error) {
	return s.store.GetByID(ctx, id)
}

func (s *MemoryService) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func (s *MemoryService) Clean(ctx context.Context) (int64, error) {
	return s.store.DeleteAll(ctx)
}

func newID() string {
	return ulid.Make().String()
}
