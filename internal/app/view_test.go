package app

import (
	"strings"
	"testing"
	"time"

	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/model"
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

func TestAnalyticsViewPrioritizesResultsAndShowsMetricState(t *testing.T) {
	cfg := config.Default()
	m := Model{
		width:              80,
		height:             24,
		mode:               modeAnalytics,
		cfg:                cfg,
		keys:               resolveKeys(cfg.Keybinds),
		filtered:           []model.Session{{ID: "one", Agent: "opencode", Project: "project", UpdatedAt: time.Now(), TokensIn: 10, TokensOut: 5, Cost: 0.25}},
		analyticsMetrics:   []bool{true, true, false, false, false, true, false, false, false},
		analyticsDimension: 1,
	}
	view := m.View()
	for _, want := range []string{"Analytics", "group:agent", "metric:", "period:", "rows", "Total", "configure"} {
		if !strings.Contains(view, want) {
			t.Fatalf("analytics view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Analytics Configuration") {
		t.Fatalf("results view unexpectedly opened configuration:\n%s", view)
	}
	if strings.Contains(view, "grain:") || strings.Count(view, "configure") != 1 {
		t.Fatalf("analytics hints or labels are duplicated/obsolete:\n%s", view)
	}
}

func TestAnalyticsConfigurationFitsCompactViewport(t *testing.T) {
	cfg := config.Default()
	m := Model{
		width:               70,
		height:              18,
		mode:                modeAnalytics,
		cfg:                 cfg,
		keys:                resolveKeys(cfg.Keybinds),
		analyticsConfigOpen: true,
		analyticsMetrics:    []bool{true, true, true, false, false, false, false, false, false},
	}
	view := m.View()
	if !strings.Contains(view, "Analytics Configuration") || !strings.Contains(view, "Group by") || !strings.Contains(view, "Metrics") || !strings.Contains(view, "period:") {
		t.Fatalf("configuration overlay incomplete:\n%s", view)
	}
	if strings.Contains(view, "grain:") {
		t.Fatalf("configuration hints or labels are duplicated/obsolete:\n%s", view)
	}
}

func TestAnalyticsViewCycleHasNoDuplicateTimelineView(t *testing.T) {
	if analyticsViewCount != 3 {
		t.Fatalf("analytics view count = %d, want table/bars/sparkline plus sentinel", analyticsViewCount)
	}
	if analyticsViewName(analyticsViewBar) != "bars" || analyticsViewName(analyticsViewSparkline) == "timeline" {
		t.Fatalf("unexpected analytics view names")
	}
}
