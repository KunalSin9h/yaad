package notifier

import (
	"context"
	"os/exec"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
)

// Compile-time interface check.
var _ ports.NotifierPort = (*NotifySend)(nil)

// NotifySend implements ports.NotifierPort using the notify-send command (Linux).
// Falls back to Stdout if notify-send is unavailable or fails.
type NotifySend struct {
	fallback *Stdout
}

func NewNotifySend() *NotifySend {
	return &NotifySend{fallback: NewStdout()}
}

// IsAvailable returns true if notify-send is installed on the system.
func IsAvailable() bool {
	_, err := exec.LookPath("notify-send")
	return err == nil
}

func (n *NotifySend) Notify(ctx context.Context, m *domain.Memory) error {
	title := "yaad reminder"
	body := m.Content
	if m.ForLabel != "" {
		body += "\n" + m.ForLabel
	}

	cmd := exec.CommandContext(ctx, "notify-send", "-u", "normal", title, body)
	if err := cmd.Run(); err != nil {
		return n.fallback.Notify(ctx, m)
	}
	return nil
}
