package timeparser

import (
	"fmt"
	"time"

	"github.com/kunalsin9h/lore/internal/domain"
	"github.com/kunalsin9h/lore/internal/ports"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
)

// Compile-time interface check.
var _ ports.TimeParserPort = (*WhenParser)(nil)

// WhenParser implements ports.TimeParserPort using github.com/olebedev/when.
type WhenParser struct {
	w *when.Parser
}

func New() *WhenParser {
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)
	return &WhenParser{w: w}
}

func (p *WhenParser) Parse(expr string, from time.Time) (*time.Time, error) {
	result, err := p.w.Parse(expr, from)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidRemindExpr, err)
	}
	if result == nil {
		return nil, fmt.Errorf("%w: %q", domain.ErrInvalidRemindExpr, expr)
	}
	return &result.Time, nil
}
