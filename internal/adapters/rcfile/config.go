// Package rcfile implements ConfigPort backed by a ~/.yaadrc file.
// Format: one "key = value" pair per line; lines starting with # are comments.
// Set() does an in-place update — it rewrites the matching line, or appends
// a new one if the key is not present yet. Comments and blank lines are
// preserved on rewrite.
package rcfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kunalsin9h/yaad/internal/ports"
)

// DefaultConfig is written by Init() when no rc file exists yet.
const DefaultConfig = `# yaad configuration
# Reference: https://github.com/KunalSin9h/yaad
#
# Flag equivalents override these values per-invocation:
#   --ollama-url   --chat-model   --embed-model

# Ollama server
ollama.url           = http://localhost:11434

# Model used to generate embeddings for semantic search.
# Recommended: nomic-embed-text, mxbai-embed-large, all-minilm
ollama.embed_model   = nomic-embed-text

# Model used for type detection, tag extraction, and query answering.
# Any chat-capable Ollama model works. Smaller = faster.
# Recommended: llama3.2:3b, mistral, gemma2:2b, phi3
ollama.chat_model    = llama3.2:3b

# How often the reminder daemon polls for due reminders.
reminder.poll_interval = 30s

# Notifier adapters to use for reminders (comma-separated, all fire together).
# cli         — prints a styled box to the terminal (default, no dependencies)
# notify-send — desktop notification via notify-send (Linux, must be installed)
notifier = cli
`

// Compile-time interface check.
var _ ports.ConfigPort = (*Config)(nil)

// Config implements ports.ConfigPort by reading and writing a plain-text
// key = value file at Path.
type Config struct {
	Path string
}

func New(path string) *Config {
	return &Config{Path: path}
}

// Init writes DefaultConfig to Path if the file does not already exist.
func (c *Config) Init() error {
	if _, err := os.Stat(c.Path); err == nil {
		return nil // already exists, do not overwrite
	}
	return os.WriteFile(c.Path, []byte(DefaultConfig), 0o644)
}

func (c *Config) Get(key string) (string, error) {
	all, err := c.All()
	if err != nil {
		return "", err
	}
	return all[key], nil
}

func (c *Config) Set(key, value string) error {
	lines, err := readLines(c.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	updated := false
	for i, line := range lines {
		k, _, ok := parseLine(line)
		if ok && k == key {
			lines[i] = fmt.Sprintf("%s = %s", key, value)
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, fmt.Sprintf("%s = %s", key, value))
	}

	return os.WriteFile(c.Path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func (c *Config) All() (map[string]string, error) {
	lines, err := readLines(c.Path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, line := range lines {
		if k, v, ok := parseLine(line); ok {
			result[k] = v
		}
	}
	return result, nil
}

// readLines reads the file and returns its lines without trailing newline on each.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// parseLine parses a single "key = value" line.
// Returns ok=false for blank lines and comment lines.
func parseLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	idx := strings.IndexByte(trimmed, '=')
	if idx < 1 {
		return "", "", false
	}
	key = strings.TrimSpace(trimmed[:idx])
	value = strings.TrimSpace(trimmed[idx+1:])
	return key, value, true
}
