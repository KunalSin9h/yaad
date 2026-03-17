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
	RemindExpr string            // natural language, e.g. "in 30 minutes"
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
// AI enrichment (embedding, type detection, tag extraction, entity extraction)
// runs in parallel and is non-fatal — the memory is saved even if Ollama is
// unavailable.
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

	// Extract and persist entities after the memory is saved.
	// Non-blocking: entity extraction failure never aborts the add.
	go func() {
		entities, err := s.ai.ExtractEntities(context.Background(), req.Content)
		if err != nil || len(entities) == 0 {
			return
		}
		if err := s.store.SaveEntities(context.Background(), m.ID, entities); err != nil {
			fmt.Fprintf(os.Stderr, "warn: save entities: %v\n", err)
		}
	}()

	return m, nil
}

// Ask answers a natural language question using hybrid retrieval + optional reranking.
//
// Retrieval pipeline:
//  1. HyDE: expand the question into a hypothetical answer, embed that.
//  2. FindHybrid: merge BM25 (FTS5) and cosine similarity via RRF.
//  3. Rerank: cross-encoder re-scores the top candidates (if configured).
//  4. Answer: LLM synthesises a response from the top memories.
func (s *MemoryService) Ask(ctx context.Context, question string) (string, error) {
	// Step 1: HyDE — embed a hypothetical answer instead of the raw question.
	// Falls back to the original question when Ollama is unavailable.
	queryText, err := s.ai.ExpandQuery(ctx, question)
	if err != nil {
		queryText = question
	}

	emb, err := s.ai.Embed(ctx, queryText)
	if err != nil {
		return "", fmt.Errorf("embed question: %w", err)
	}

	// Step 2: Hybrid BM25 + vector retrieval with RRF fusion.
	// Fetch a larger pool (10) before reranking down to 5.
	memories, err := s.store.FindHybrid(ctx, question, emb, 10)
	if err != nil {
		return "", fmt.Errorf("find hybrid: %w", err)
	}
	if len(memories) == 0 {
		return "No relevant memories found.", nil
	}

	// Step 3: Rerank — cross-encoder re-orders candidates by true relevance.
	// No-op when rerank model is not configured; never blocks on error.
	memories, err = s.ai.Rerank(ctx, question, memories)
	if err != nil {
		// Non-fatal: use the hybrid-ranked order.
		fmt.Fprintf(os.Stderr, "warn: rerank: %v\n", err)
	}

	// Use top 5 for answer synthesis.
	if len(memories) > 5 {
		memories = memories[:5]
	}

	// Step 4: LLM-synthesised answer.
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
