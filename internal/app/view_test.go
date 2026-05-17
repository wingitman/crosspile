package app

import (
	"strings"
	"testing"
	"time"

	"github.com/wingitman/crosspile/internal/config"
)

func TestViewRendersWarningDetails(t *testing.T) {
	cfg := config.Default()
	m := Model{
		width:    100,
		height:   24,
		mode:     modeNormal,
		keys:     resolveKeys(cfg.Keybinds),
		lastScan: time.Date(2026, 5, 17, 12, 29, 59, 0, time.UTC),
		warnings: []string{"opencode: no such table: session"},
	}

	view := m.View()
	for _, want := range []string{"1 warning(s)", "warnings:", "opencode: no such table: session"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}
