package notifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kunalsin9h/lore/internal/domain"
	"github.com/kunalsin9h/lore/internal/ports"
)

// Compile-time interface check.
var _ ports.NotifierPort = (*CLI)(nil)

const (
	ansiYellow = "\033[33m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiReset  = "\033[0m"
)

// CLI implements ports.NotifierPort by printing a styled box to stdout.
// Works on any terminal, no system dependencies.
type CLI struct{}

func NewCLI() *CLI { return &CLI{} }

func (c *CLI) Notify(_ context.Context, m *domain.Memory) error {
	lines := []string{m.Content}
	if m.ForLabel != "" {
		lines = append(lines, ansiDim+"for: "+m.ForLabel+ansiReset)
	}
	if m.RemindAt != nil {
		lines = append(lines, ansiDim+"due: "+m.RemindAt.Format(time.RFC822)+ansiReset)
	}

	// Compute box width from visible (non-ANSI) content.
	width := len("lore · reminder") // minimum based on header
	for _, l := range lines {
		if vl := visibleLen(l); vl > width {
			width = vl
		}
	}
	width += 2 // padding

	bar := strings.Repeat("─", width+2)
	fmt.Println()
	fmt.Printf(ansiYellow+"  ┌"+bar+"┐"+ansiReset+"\n")
	fmt.Printf(ansiYellow+"  │ "+ansiReset+ansiBold+"%-*s"+ansiReset+ansiYellow+" │"+ansiReset+"\n", width, "lore · reminder")
	fmt.Printf("%s  │ %s%-*s%s │%s\n", ansiYellow, ansiReset, width, "", ansiYellow, ansiReset)
	for _, l := range lines {
		pad := width - visibleLen(l)
		fmt.Printf("%s  │ %s%s%s%s │%s\n", ansiYellow, ansiReset, l, strings.Repeat(" ", pad), ansiYellow, ansiReset)
	}
	fmt.Printf(ansiYellow+"  └"+bar+"┘"+ansiReset+"\n")
	fmt.Println()
	return nil
}

// visibleLen returns the length of s excluding ANSI escape sequences.
func visibleLen(s string) int {
	n := 0
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
		} else if inEsc && s[i] == 'm' {
			inEsc = false
		} else if !inEsc {
			n++
		}
	}
	return n
}
