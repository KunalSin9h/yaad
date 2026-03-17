package ports

import (
	"context"

	"github.com/kunalsin9h/yaad/internal/domain"
)

// AIPort defines all intelligence operations.
// Implemented by: adapters/ollama.Client
//
// All methods must degrade gracefully — callers should be able to proceed
// with reduced quality when Ollama is unavailable or a model is not configured.
type AIPort interface {
	// Core enrichment (existing).
	Embed(ctx context.Context, text string) ([]float32, error)
	DetectType(ctx context.Context, content string) (domain.MemoryType, error)
	ExtractTags(ctx context.Context, content, forLabel string) ([]string, error)
	Answer(ctx context.Context, question string, memories []*domain.Memory) (string, error)

	// ExpandQuery uses HyDE (Hypothetical Document Embeddings): it generates a
	// short hypothetical passage that would answer the question, then embeds
	// that passage instead of the raw question. This dramatically improves
	// semantic recall for abstract or indirect questions.
	// Returns the original question on any error so the caller can always embed.
	ExpandQuery(ctx context.Context, question string) (string, error)

	// Rerank re-orders candidates by cross-encoder relevance to query.
	// Uses Qwen3-Reranker via Ollama (model configured separately as rerank_model).
	// Returns candidates in original order when the rerank model is unavailable.
	Rerank(ctx context.Context, query string, candidates []*domain.Memory) ([]*domain.Memory, error)

	// ExtractEntities extracts named entities (people, projects, tools, concepts,
	// places) from content. Used to populate the knowledge graph.
	// Returns an empty slice on any error so the caller can save the memory anyway.
	ExtractEntities(ctx context.Context, content string) ([]domain.Entity, error)
}
