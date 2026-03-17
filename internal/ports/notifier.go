package ports

import (
	"context"

	"github.com/kunalsin9h/lore/internal/domain"
)

// NotifierPort delivers reminder alerts to the user.
// Implemented by: adapters/notifier.Stdout, adapters/notifier.NotifySend
type NotifierPort interface {
	Notify(ctx context.Context, m *domain.Memory) error
}
