package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	sqliteadapter "github.com/kunalsin9h/yaad/internal/adapters/sqlite"
	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sqliteadapter.DB {
	t.Helper()
	db, err := sqliteadapter.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func sampleMemory(id string) *domain.Memory {
	return &domain.Memory{
		ID:         id,
		Content:    "sample content " + id,
		ForLabel:   "sample for label",
		WorkingDir: "/home/test",
		Hostname:   "testhost",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
}

// --- Save / GetByID ---

func TestStore_SaveAndGetByID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	m := sampleMemory("01SAVE01")

	require.NoError(t, db.Store.Save(ctx, m))

	got, err := db.Store.GetByID(ctx, "01SAVE01")
	require.NoError(t, err)

	assert.Equal(t, m.ID, got.ID)
	assert.Equal(t, m.Content, got.Content)
	assert.Equal(t, m.ForLabel, got.ForLabel)
	assert.Equal(t, m.WorkingDir, got.WorkingDir)
	assert.Equal(t, m.Hostname, got.Hostname)
}

func TestStore_Save_Upsert(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	m := sampleMemory("01UPSERT")
	require.NoError(t, db.Store.Save(ctx, m))

	m.Content = "updated content"
	require.NoError(t, db.Store.Save(ctx, m))

	got, err := db.Store.GetByID(ctx, "01UPSERT")
	require.NoError(t, err)
	assert.Equal(t, "updated content", got.Content)
}

func TestStore_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := db.Store.GetByID(context.Background(), "notexist")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStore_Save_WithRemindAt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	m := sampleMemory("01REMIND")
	remindAt := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	m.RemindAt = &remindAt

	require.NoError(t, db.Store.Save(ctx, m))

	got, err := db.Store.GetByID(ctx, "01REMIND")
	require.NoError(t, err)
	require.NotNil(t, got.RemindAt)
	assert.WithinDuration(t, remindAt, *got.RemindAt, time.Second)
}

// --- List ---

func TestStore_List_All(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	for _, id := range []string{"01LIST1", "01LIST2", "01LIST3"} {
		require.NoError(t, db.Store.Save(ctx, sampleMemory(id)))
	}

	results, err := db.Store.List(ctx, domain.ListFilter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestStore_List_OnlyReminders(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	plain := sampleMemory("01PLAIN1")
	reminder := sampleMemory("01REMND1")
	t1 := time.Now().Add(1 * time.Hour)
	reminder.RemindAt = &t1
	require.NoError(t, db.Store.Save(ctx, plain))
	require.NoError(t, db.Store.Save(ctx, reminder))

	results, err := db.Store.List(ctx, domain.ListFilter{OnlyReminders: true})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "01REMND1", results[0].ID)
}

func TestStore_List_Limit(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	for _, id := range []string{"01LIM001", "01LIM002", "01LIM003", "01LIM004", "01LIM005"} {
		require.NoError(t, db.Store.Save(ctx, sampleMemory(id)))
	}

	results, err := db.Store.List(ctx, domain.ListFilter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

// --- Delete ---

func TestStore_Delete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.Store.Save(ctx, sampleMemory("01DEL001")))

	require.NoError(t, db.Store.Delete(ctx, "01DEL001"))

	_, err := db.Store.GetByID(ctx, "01DEL001")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStore_Delete_NotFound(t *testing.T) {
	db := newTestDB(t)
	err := db.Store.Delete(context.Background(), "doesnotexist")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// --- FindSimilar ---

func TestStore_FindSimilar_RanksByCosineSimilarity(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Two memories: one with a very similar embedding, one orthogonal.
	close := sampleMemory("01CLOSE1")
	close.Embedding = []float32{1.0, 0.0, 0.0}

	far := sampleMemory("01FAR001")
	far.Embedding = []float32{0.0, 1.0, 0.0}

	require.NoError(t, db.Store.Save(ctx, close))
	require.NoError(t, db.Store.Save(ctx, far))

	// Query embedding almost identical to "close".
	results, err := db.Store.FindSimilar(ctx, []float32{0.99, 0.01, 0.0}, 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "01CLOSE1", results[0].ID, "closest embedding should rank first")
}

func TestStore_FindSimilar_SkipsEmptyEmbeddings(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	withEmb := sampleMemory("01WITHEMB")
	withEmb.Embedding = []float32{1.0, 0.0}
	noEmb := sampleMemory("01NOEMBED")
	noEmb.Embedding = nil

	require.NoError(t, db.Store.Save(ctx, withEmb))
	require.NoError(t, db.Store.Save(ctx, noEmb))

	results, err := db.Store.FindSimilar(ctx, []float32{1.0, 0.0}, 5)
	require.NoError(t, err)
	assert.Len(t, results, 1, "memory without embedding should be excluded")
}

// --- Reminders ---

func TestStore_PendingReminders(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	due := sampleMemory("01DUE001")
	dueAt := time.Now().Add(-1 * time.Minute) // already past
	due.RemindAt = &dueAt

	future := sampleMemory("01FUT001")
	futureAt := time.Now().Add(1 * time.Hour) // not yet
	future.RemindAt = &futureAt

	plain := sampleMemory("01PLN001")

	require.NoError(t, db.Store.Save(ctx, due))
	require.NoError(t, db.Store.Save(ctx, future))
	require.NoError(t, db.Store.Save(ctx, plain))

	results, err := db.Store.PendingReminders(ctx, time.Now())
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "01DUE001", results[0].ID)
}

func TestStore_MarkReminded(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	m := sampleMemory("01MARKR1")
	dueAt := time.Now().Add(-1 * time.Minute)
	m.RemindAt = &dueAt
	require.NoError(t, db.Store.Save(ctx, m))

	require.NoError(t, db.Store.MarkReminded(ctx, "01MARKR1"))

	// Should no longer appear in pending reminders.
	pending, err := db.Store.PendingReminders(ctx, time.Now())
	require.NoError(t, err)
	assert.Empty(t, pending)

	// RemindedAt should now be set.
	got, err := db.Store.GetByID(ctx, "01MARKR1")
	require.NoError(t, err)
	assert.NotNil(t, got.RemindedAt)
}
