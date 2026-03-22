package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	sqliteadapter "github.com/kunalsin9h/yaad/internal/adapters/sqlite"
	"github.com/kunalsin9h/yaad/internal/app"
	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemoryService(t *testing.T, ai *testutil.MockAI, timer *testutil.MockTimeParser) (*app.MemoryService, *sqliteadapter.DB) {
	t.Helper()
	db, err := sqliteadapter.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	svc := app.NewMemoryService(db.Store, ai, timer)
	return svc, db
}

// --- Add ---

func TestMemoryService_Add_StoresContent(t *testing.T) {
	svc, db := newMemoryService(t, &testutil.MockAI{}, &testutil.MockTimeParser{})

	m, err := svc.Add(context.Background(), app.AddRequest{
		Content:  "claude --resume abc123",
		ForLabel: "yaad build session",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, m.ID)
	assert.Equal(t, "claude --resume abc123", m.Content)
	assert.Equal(t, "yaad build session", m.ForLabel)

	// Verify it was persisted.
	got, err := db.Store.GetByID(context.Background(), m.ID)
	require.NoError(t, err)
	assert.Equal(t, m.Content, got.Content)
}

func TestMemoryService_Add_CapturesSystemContext(t *testing.T) {
	svc, _ := newMemoryService(t, &testutil.MockAI{}, &testutil.MockTimeParser{})

	m, err := svc.Add(context.Background(), app.AddRequest{Content: "test"})
	require.NoError(t, err)

	assert.NotEmpty(t, m.WorkingDir, "WorkingDir should be captured automatically")
	assert.NotEmpty(t, m.Hostname, "Hostname should be captured automatically")
}

func TestMemoryService_Add_SetsCreatedAt(t *testing.T) {
	svc, _ := newMemoryService(t, &testutil.MockAI{}, &testutil.MockTimeParser{})
	before := time.Now()
	m, err := svc.Add(context.Background(), app.AddRequest{Content: "test"})
	require.NoError(t, err)
	assert.True(t, m.CreatedAt.After(before) || m.CreatedAt.Equal(before))
}

func TestMemoryService_Add_WithRemindExpr_SetsRemindAt(t *testing.T) {
	wantRemind := time.Now().Add(30 * time.Minute)
	timer := &testutil.MockTimeParser{
		ParseFn: func(expr string, from time.Time) (*time.Time, error) {
			t := wantRemind
			return &t, nil
		},
	}
	svc, _ := newMemoryService(t, &testutil.MockAI{}, timer)

	m, err := svc.Add(context.Background(), app.AddRequest{
		Content:    "book conference ticket",
		RemindExpr: "in 30 minutes",
	})
	require.NoError(t, err)
	require.NotNil(t, m.RemindAt)
	assert.WithinDuration(t, wantRemind, *m.RemindAt, time.Second)
}

func TestMemoryService_Add_InvalidRemindExpr_ReturnsError(t *testing.T) {
	timer := &testutil.MockTimeParser{
		ParseFn: func(expr string, from time.Time) (*time.Time, error) {
			return nil, domain.ErrInvalidRemindExpr
		},
	}
	svc, _ := newMemoryService(t, &testutil.MockAI{}, timer)

	_, err := svc.Add(context.Background(), app.AddRequest{
		Content:    "test",
		RemindExpr: "not a real time",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidRemindExpr)
}

func TestMemoryService_Add_AIFailure_StillSaves(t *testing.T) {
	ai := &testutil.MockAI{
		EmbedFn: func(_ context.Context, _ string) ([]float32, error) {
			return nil, errors.New("ollama not running")
		},
	}
	svc, db := newMemoryService(t, ai, &testutil.MockTimeParser{})

	m, err := svc.Add(context.Background(), app.AddRequest{Content: "important note"})
	require.NoError(t, err, "Add should succeed even when all AI calls fail")
	assert.NotEmpty(t, m.ID)

	// Must be saved to DB with no embedding.
	got, err := db.Store.GetByID(context.Background(), m.ID)
	require.NoError(t, err)
	assert.Equal(t, "important note", got.Content)
	assert.Empty(t, got.Embedding)
}

// --- Ask ---

func TestMemoryService_Ask_NoMemories_ReturnsMessage(t *testing.T) {
	ai := &testutil.MockAI{
		EmbedFn: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1.0, 0.0}, nil
		},
	}
	svc, _ := newMemoryService(t, ai, &testutil.MockTimeParser{})

	answer, err := svc.Ask(context.Background(), "what was that command?")
	require.NoError(t, err)
	assert.NotEmpty(t, answer)
}

func TestMemoryService_Ask_CallsAnswerWithRelevantMemories(t *testing.T) {
	var answeredWith []*domain.Memory
	ai := &testutil.MockAI{
		EmbedFn: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1.0, 0.0}, nil
		},
		AnswerFn: func(_ context.Context, _ string, memories []*domain.Memory) (string, error) {
			answeredWith = memories
			return "the answer", nil
		},
	}
	svc, _ := newMemoryService(t, ai, &testutil.MockTimeParser{})

	// Save a memory with an embedding so FindSimilar can return it.
	_, err := svc.Add(context.Background(), app.AddRequest{Content: "claude --resume abc"})
	require.NoError(t, err)

	_, err = svc.Ask(context.Background(), "what was that claude command?")
	require.NoError(t, err)
	assert.NotEmpty(t, answeredWith, "Answer should be called with recalled memories")
}

// --- List ---

func TestMemoryService_List(t *testing.T) {
	svc, _ := newMemoryService(t, &testutil.MockAI{}, &testutil.MockTimeParser{})
	ctx := context.Background()

	for _, c := range []string{"note one", "note two", "note three"} {
		_, err := svc.Add(ctx, app.AddRequest{Content: c})
		require.NoError(t, err)
	}

	results, err := svc.List(ctx, domain.ListFilter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

// --- Delete ---

func TestMemoryService_Delete(t *testing.T) {
	svc, _ := newMemoryService(t, &testutil.MockAI{}, &testutil.MockTimeParser{})
	ctx := context.Background()

	m, err := svc.Add(ctx, app.AddRequest{Content: "to be deleted"})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, m.ID))

	_, err = svc.GetByID(ctx, m.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMemoryService_Delete_NotFound(t *testing.T) {
	svc, _ := newMemoryService(t, &testutil.MockAI{}, &testutil.MockTimeParser{})
	err := svc.Delete(context.Background(), "doesnotexist")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
