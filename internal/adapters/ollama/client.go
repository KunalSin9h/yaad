package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/kunalsin9h/lore/internal/domain"
	"github.com/kunalsin9h/lore/internal/ports"
)

// Compile-time interface check.
var _ ports.AIPort = (*Client)(nil)

// Client implements ports.AIPort using the Ollama REST API directly.
// Using the HTTP API avoids pulling the entire ollama module as a dependency.
type Client struct {
	baseURL    string
	embedModel string
	chatModel  string
	http       *http.Client
}

func New(baseURL, embedModel, chatModel string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		embedModel: embedModel,
		chatModel:  chatModel,
		http:       &http.Client{},
	}
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

	// Extract the JSON array from the response robustly.
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
