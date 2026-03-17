package rcfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kunalsin9h/yaad/internal/adapters/rcfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTempConfig(t *testing.T, content string) *rcfile.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".yaadrc")
	if content != "" {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return rcfile.New(path)
}

// --- Get ---

func TestGet_ExistingKey(t *testing.T) {
	cfg := newTempConfig(t, "ollama.url = http://localhost:11434\n")
	v, err := cfg.Get("ollama.url")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", v)
}

func TestGet_TrimsWhitespace(t *testing.T) {
	cfg := newTempConfig(t, "  ollama.url   =   http://localhost:11434  \n")
	v, err := cfg.Get("ollama.url")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", v)
}

func TestGet_MissingKey_ReturnsEmpty(t *testing.T) {
	cfg := newTempConfig(t, "ollama.url = http://localhost:11434\n")
	v, err := cfg.Get("does.not.exist")
	require.NoError(t, err)
	assert.Empty(t, v)
}

func TestGet_CommentLines_Skipped(t *testing.T) {
	cfg := newTempConfig(t, "# ollama.url = http://shouldbeskipped\nollama.url = http://real\n")
	v, err := cfg.Get("ollama.url")
	require.NoError(t, err)
	assert.Equal(t, "http://real", v)
}

func TestGet_MissingFile_ReturnsEmpty(t *testing.T) {
	cfg := rcfile.New(filepath.Join(t.TempDir(), "nonexistent.rc"))
	v, err := cfg.Get("any.key")
	require.NoError(t, err)
	assert.Empty(t, v)
}

// --- Set ---

func TestSet_NewKey_AppendsToFile(t *testing.T) {
	cfg := newTempConfig(t, "ollama.url = http://localhost:11434\n")
	require.NoError(t, cfg.Set("ollama.chat_model", "llama3.2:3b"))

	v, err := cfg.Get("ollama.chat_model")
	require.NoError(t, err)
	assert.Equal(t, "llama3.2:3b", v)

	// Original key must still be present.
	u, err := cfg.Get("ollama.url")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", u)
}

func TestSet_ExistingKey_UpdatesInPlace(t *testing.T) {
	cfg := newTempConfig(t, "ollama.chat_model = llama3.2:3b\n")
	require.NoError(t, cfg.Set("ollama.chat_model", "mistral"))

	v, err := cfg.Get("ollama.chat_model")
	require.NoError(t, err)
	assert.Equal(t, "mistral", v)
}

func TestSet_PreservesComments(t *testing.T) {
	content := "# my favourite model\nollama.chat_model = llama3.2:3b\n"
	cfg := newTempConfig(t, content)
	require.NoError(t, cfg.Set("ollama.chat_model", "mistral"))

	raw, err := os.ReadFile(cfg.Path)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(raw), "# my favourite model"),
		"comment should be preserved after Set()")
}

func TestSet_CreatesFile_WhenMissing(t *testing.T) {
	cfg := rcfile.New(filepath.Join(t.TempDir(), ".yaadrc"))
	require.NoError(t, cfg.Set("ollama.url", "http://localhost:11434"))

	v, err := cfg.Get("ollama.url")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", v)
}

// --- All ---

func TestAll_ReturnsAllPairs(t *testing.T) {
	cfg := newTempConfig(t, "# comment\nollama.url = http://localhost:11434\nollama.chat_model = mistral\n")
	all, err := cfg.All()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ollama.url":        "http://localhost:11434",
		"ollama.chat_model": "mistral",
	}, all)
}

func TestAll_MissingFile_ReturnsEmpty(t *testing.T) {
	cfg := rcfile.New(filepath.Join(t.TempDir(), "nonexistent.rc"))
	all, err := cfg.All()
	require.NoError(t, err)
	assert.Empty(t, all)
}

// --- Init ---

func TestInit_CreatesFileWithDefaults(t *testing.T) {
	cfg := rcfile.New(filepath.Join(t.TempDir(), ".yaadrc"))
	require.NoError(t, cfg.Init())

	raw, err := os.ReadFile(cfg.Path)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(raw), "ollama.url"),
		"default config should contain ollama.url")
}

func TestInit_IsIdempotent(t *testing.T) {
	cfg := newTempConfig(t, "ollama.url = http://custom\n")
	original, err := os.ReadFile(cfg.Path)
	require.NoError(t, err)

	// Init on an existing file must not overwrite it.
	require.NoError(t, cfg.Init())

	after, err := os.ReadFile(cfg.Path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(after))
}
