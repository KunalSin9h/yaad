package app

import (
	"context"
	"fmt"
	"time"

	"github.com/kunalsin9h/yaad/internal/ports"
)

// ReminderService checks for due reminders and fires notifications.
type ReminderService struct {
	store    ports.StoragePort
	notifier ports.NotifierPort
}

func NewReminderService(store ports.StoragePort, notifier ports.NotifierPort) *ReminderService {
	return &ReminderService{store: store, notifier: notifier}
}

// CheckAndFire fires all reminders due before now and marks them as reminded.
func (s *ReminderService) CheckAndFire(ctx context.Context) error {
	memories, err := s.store.PendingReminders(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("fetch pending reminders: %w", err)
	}

	for _, m := range memories {
		if err := s.notifier.Notify(ctx, m); err != nil {
			// Log but continue firing remaining reminders.
			fmt.Printf("warn: notify %s: %v\n", m.ID, err)
		}
		if err := s.store.MarkReminded(ctx, m.ID); err != nil {
			return fmt.Errorf("mark reminded %s: %w", m.ID, err)
		}
	}
	return nil
}

// RunDaemon runs CheckAndFire on the given interval until ctx is cancelled.
// Designed to be managed by systemd --user as a persistent background service.
func (s *ReminderService) RunDaemon(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Fire immediately on start so there is no delay after launch.
	if err := s.CheckAndFire(ctx); err != nil {
		fmt.Printf("warn: initial check: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.CheckAndFire(ctx); err != nil {
				fmt.Printf("warn: check: %v\n", err)
			}
		}
	}
}
