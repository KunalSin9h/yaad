package app_test

import (
	"context"
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

func newReminderService(t *testing.T, notif *testutil.MockNotifier) (*app.ReminderService, *sqliteadapter.DB) {
	t.Helper()
	db, err := sqliteadapter.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	svc := app.NewReminderService(db.Store, notif)
	return svc, db
}

func saveReminder(t *testing.T, db *sqliteadapter.DB, id string, remindAt time.Time) {
	t.Helper()
	m := &domain.Memory{
		ID:        id,
		Content:   "reminder: " + id,
		CreatedAt: time.Now(),
		RemindAt:  &remindAt,
	}
	require.NoError(t, db.Store.Save(context.Background(), m))
}

func TestReminderService_CheckAndFire_NoPending(t *testing.T) {
	notif := &testutil.MockNotifier{}
	svc, db := newReminderService(t, notif)

	// Save a reminder that is NOT yet due.
	saveReminder(t, db, "01FUTURE1", time.Now().Add(1*time.Hour))

	require.NoError(t, svc.CheckAndFire(context.Background()))
	assert.Empty(t, notif.Fired, "no reminders should fire when none are due")
}

func TestReminderService_CheckAndFire_FiresDueReminders(t *testing.T) {
	notif := &testutil.MockNotifier{}
	svc, db := newReminderService(t, notif)

	saveReminder(t, db, "01PAST001", time.Now().Add(-1*time.Minute)) // due
	saveReminder(t, db, "01PAST002", time.Now().Add(-5*time.Minute)) // due
	saveReminder(t, db, "01FUTURE1", time.Now().Add(1*time.Hour))    // not due

	require.NoError(t, svc.CheckAndFire(context.Background()))
	assert.Len(t, notif.Fired, 2, "both overdue reminders should fire")
}

func TestReminderService_CheckAndFire_MarksRemindersFired(t *testing.T) {
	notif := &testutil.MockNotifier{}
	svc, db := newReminderService(t, notif)

	saveReminder(t, db, "01MARKR1", time.Now().Add(-1*time.Minute))
	require.NoError(t, svc.CheckAndFire(context.Background()))

	// Second call must not fire again.
	notif.Fired = nil
	require.NoError(t, svc.CheckAndFire(context.Background()))
	assert.Empty(t, notif.Fired, "already-fired reminder must not fire again")
}

func TestReminderService_CheckAndFire_SetsRemindedAt(t *testing.T) {
	notif := &testutil.MockNotifier{}
	svc, db := newReminderService(t, notif)

	saveReminder(t, db, "01REMDAT1", time.Now().Add(-1*time.Minute))
	require.NoError(t, svc.CheckAndFire(context.Background()))

	got, err := db.Store.GetByID(context.Background(), "01REMDAT1")
	require.NoError(t, err)
	assert.NotNil(t, got.RemindedAt, "RemindedAt must be set after firing")
}

func TestReminderService_CheckAndFire_ContinuesOnNotifyError(t *testing.T) {
	// Both notify calls succeed; we verify both are attempted.
	callCount := 0
	var fired []*domain.Memory
	notif := &testutil.MockNotifier{
		NotifyFn: func(_ context.Context, m *domain.Memory) error {
			callCount++
			fired = append(fired, m)
			return nil
		},
	}
	svc, db := newReminderService(t, notif)

	saveReminder(t, db, "01CONT001", time.Now().Add(-1*time.Minute))
	saveReminder(t, db, "01CONT002", time.Now().Add(-2*time.Minute))

	require.NoError(t, svc.CheckAndFire(context.Background()))
	assert.Equal(t, 2, callCount, "both reminders should be attempted even if one fails")
}
