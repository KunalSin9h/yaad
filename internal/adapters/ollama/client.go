package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
)

// Compile-time interface check.
var _ ports.AIPort = (*Client)(nil)

// Client implements ports.AIPort using the Ollama REST API directly.
// Using the HTTP API avoids pulling the entire ollama module as a dependency.
type Client struct {
	baseURL      string
	embedModel   string
	chatModel    string
	rerankModel  string // optional; empty = skip reranking
	http         *http.Client
}

func New(baseURL, embedModel, chatModel string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		embedModel: embedModel,
		chatModel:  chatModel,
		http:       &http.Client{},
	}
}

// WithRerankModel configures an optional Qwen3-Reranker model for cross-encoder
// reranking. Example: "dengcao/Qwen3-Reranker-0.6B".
// When empty, Rerank() is a no-op that returns candidates in original order.
func (c *Client) WithRerankModel(model string) *Client {
	c.rerankModel = model
	return c
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]string{
		"model":  c.embedModel,
		"prompt": text,
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrOllamaUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", domain.ErrOllamaUnavailable, resp.StatusCode)
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	out := make([]float32, len(result.Embedding))
	for i, v := range result.Embedding {
		out[i] = float32(v)
	}
	return out, nil
}

func (c *Client) DetectType(ctx context.Context, content string) (domain.MemoryType, error) {
	prompt := fmt.Sprintf(
		`Classify the following text into exactly one category: command, note, reminder, url, fact.
Reply with only the single word category, nothing else.
Text: %q`,
		content,
	)

	result, err := c.chat(ctx, prompt)
	if err != nil {
		return domain.MemoryTypeNote, err
	}

	switch strings.TrimSpace(strings.ToLower(result)) {
	case "command":
		return domain.MemoryTypeCommand, nil
	case "reminder":
		return domain.MemoryTypeReminder, nil
	case "url":
		return domain.MemoryTypeURL, nil
	case "fact":
		return domain.MemoryTypeFact, nil
	default:
		return domain.MemoryTypeNote, nil
	}
}

func (c *Client) ExtractTags(ctx context.Context, content, forLabel string) ([]string, error) {
	prompt := fmt.Sprintf(
		`Extract 2-5 short keyword tags from the text below.
Return only a JSON array of lowercase strings, nothing else.
Example output: ["docker","networking","port"]
Text: %q
Context: %q`,
		content, forLabel,
	)

	result, err := c.chat(ctx, prompt)
	if err != nil {
		return nil, err
	}

	start := strings.Index(result, "[")
	end := strings.LastIndex(result, "]")
	if start == -1 || end == -1 || end <= start {
		return []string{}, nil
	}

	var tags []string
	if err := json.Unmarshal([]byte(result[start:end+1]), &tags); err != nil {
		return []string{}, nil
	}
	return tags, nil
}

func (c *Client) Answer(ctx context.Context, question string, memories []*domain.Memory) (string, error) {
	var sb strings.Builder
	sb.WriteString("You are a personal memory assistant. Answer the user's question using only their saved memories below.\n")
	sb.WriteString("Be concise and direct. If the answer is in the memories, quote the relevant content.\n\n")
	sb.WriteString("Saved memories:\n")
	for i, m := range memories {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, m.Type, m.Content))
		if m.ForLabel != "" {
			sb.WriteString(fmt.Sprintf(" (context: %s)", m.ForLabel))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("\nQuestion: %s", question))

	return c.chat(ctx, sb.String())
}

// ExpandQuery implements HyDE (Hypothetical Document Embeddings).
// It asks the LLM to write a short passage that would answer the question,
// then returns that passage to be embedded instead of the raw question.
// Embedding a hypothetical answer retrieves documents whose style and
// vocabulary match answers, not questions — improving semantic recall by
// 10–30% on abstract queries.
// Falls back to the original question on any error.
func (c *Client) ExpandQuery(ctx context.Context, question string) (string, error) {
	prompt := fmt.Sprintf(
		`Write a short, dense passage (2-4 sentences) that directly answers this question.
Write as if you know the answer. Be specific and use concrete vocabulary.
Do not say "I don't know" — make a plausible answer.
Question: %s`,
		question,
	)

	expanded, err := c.chat(ctx, prompt)
	if err != nil {
		// Graceful degradation: return original question.
		return question, nil
	}
	expanded = strings.TrimSpace(expanded)
	if expanded == "" {
		return question, nil
	}
	return expanded, nil
}

// Rerank uses Qwen3-Reranker via Ollama to perform cross-encoder relevance
// scoring. A cross-encoder sees (query, document) together, enabling nuanced
// contextual relevance assessment that bi-encoder cosine similarity misses.
//
// The Qwen3 reranker prompt format follows the model's training convention.
// Each candidate is scored independently; results are sorted descending by score.
//
// If rerankModel is empty or Ollama is unavailable, candidates are returned
// in their original order (graceful degradation).
func (c *Client) Rerank(ctx context.Context, query string, candidates []*domain.Memory) ([]*domain.Memory, error) {
	if c.rerankModel == "" || len(candidates) == 0 {
		return candidates, nil
	}

	type scored struct {
		m     *domain.Memory
		score float64
	}
	results := make([]scored, len(candidates))

	for i, m := range candidates {
		doc := m.Content
		if m.ForLabel != "" {
			doc += " — " + m.ForLabel
		}

		// Qwen3-Reranker prompt format: the model is trained to output
		// "yes" if the document answers the query, "no" otherwise.
		systemPrompt := "Judge whether the Document meets the requirements of the Query. Output your final verdict by strictly following this format: \"yes\" or \"no\"."
		userMsg := fmt.Sprintf("<Query>%s</Query>\n<Document>%s</Document>", query, doc)

		payload := map[string]any{
			"model": c.rerankModel,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userMsg},
			},
			"stream": false,
		}
		b, _ := json.Marshal(payload)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(b))
		if err != nil {
			return candidates, nil
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			// Reranker unavailable — return original order.
			return candidates, nil
		}

		var result struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
			resp.Body.Close()
			return candidates, nil
		}
		resp.Body.Close()

		answer := strings.ToLower(strings.TrimSpace(result.Message.Content))
		score := 0.0
		if strings.HasPrefix(answer, "yes") {
			score = 1.0
		}
		results[i] = scored{m: m, score: score}
	}

	// Stable sort: yes-scored items first, preserving original order within
	// each score tier (so cosine order is the tiebreaker).
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	out := make([]*domain.Memory, len(results))
	for i, r := range results {
		out[i] = r.m
	}
	return out, nil
}

// ExtractEntities extracts named entities from content.
// Returns person, place, project, concept, and tool entities as structured data.
// Used to populate the knowledge graph so memories can be connected by entity.
func (c *Client) ExtractEntities(ctx context.Context, content string) ([]domain.Entity, error) {
	prompt := fmt.Sprintf(
		`Extract named entities from the text below. Only include entities that are clearly named.
Return a JSON array. Each item must have "name" (string) and "type" (one of: person, place, project, concept, tool).
If no entities exist, return [].
Examples:
  Text: "fix nginx config for the payments-api project"
  Output: [{"name":"nginx","type":"tool"},{"name":"payments-api","type":"project"}]
Text: %q`,
		content,
	)

	result, err := c.chat(ctx, prompt)
	if err != nil {
		return []domain.Entity{}, nil
	}

	start := strings.Index(result, "[")
	end := strings.LastIndex(result, "]")
	if start == -1 || end == -1 || end <= start {
		return []domain.Entity{}, nil
	}

	var raw []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(result[start:end+1]), &raw); err != nil {
		return []domain.Entity{}, nil
	}

	entities := make([]domain.Entity, 0, len(raw))
	for _, r := range raw {
		et := domain.EntityType(r.Type)
		switch et {
		case domain.EntityTypePerson, domain.EntityTypePlace, domain.EntityTypeProject,
			domain.EntityTypeConcept, domain.EntityTypeTool:
		default:
			et = domain.EntityTypeConcept
		}
		if r.Name != "" {
			entities = append(entities, domain.Entity{
				Name: strings.ToLower(r.Name),
				Type: et,
			})
		}
	}
	return entities, nil
}

func (c *Client) chat(ctx context.Context, prompt string) (string, error) {
	payload := map[string]any{
		"model": c.chatModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %s", domain.ErrOllamaUnavailable, err)
	}
	defer resp.Body.Close()

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	return result.Message.Content, nil
}
