package notifier

import (
	"context"
	"errors"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
)

// Compile-time interface check.
var _ ports.NotifierPort = (*Multi)(nil)

// Multi fans out a Notify call to all registered notifiers.
// All notifiers are called even if one fails; errors are joined.
type Multi struct {
	notifiers []ports.NotifierPort
}

func NewMulti(notifiers ...ports.NotifierPort) *Multi {
	return &Multi{notifiers: notifiers}
}

func (m *Multi) Notify(ctx context.Context, mem *domain.Memory) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, mem); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
