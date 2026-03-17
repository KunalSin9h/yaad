package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/kunalsin9h/lore/internal/domain"
	"github.com/kunalsin9h/lore/internal/ports"
)

// Compile-time interface check.
var _ ports.NotifierPort = (*Stdout)(nil)

// Stdout implements ports.NotifierPort by printing to stdout.
// Used as the universal fallback when no desktop notifier is available.
type Stdout struct{}

func NewStdout() *Stdout { return &Stdout{} }

func (n *Stdout) Notify(_ context.Context, m *domain.Memory) error {
	fmt.Println()
	fmt.Println("*** REMINDER ***")
	fmt.Printf("  %s\n", m.Content)
	if m.ForLabel != "" {
		fmt.Printf("  Context : %s\n", m.ForLabel)
	}
	if m.RemindAt != nil {
		fmt.Printf("  Due     : %s\n", m.RemindAt.Format(time.RFC822))
	}
	fmt.Println("****************")
	fmt.Println()
	return nil
}
