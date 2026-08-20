package app

import (
	"strings"
	"testing"
	"time"

	"github.com/wingitman/crosspile/internal/analytics"
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
	for _, want := range []string{"Pivot Analytics", "rows:agent", "metric:", "period:", "rows", "Total", "configure"} {
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
	if !strings.Contains(view, "Pivot Configuration") || !strings.Contains(view, "Fields") || !strings.Contains(view, "Wells") || !strings.Contains(view, "period:") {
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

func TestPivotViewRendersHeadersCellsAndTotals(t *testing.T) {
	cfg := config.Default()
	m := Model{
		width: 80, height: 24, mode: modeAnalytics, cfg: cfg, keys: resolveKeys(cfg.Keybinds),
		filtered: []model.Session{
			{ID: "one", Agent: "alpha", Project: "p", UpdatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), TokensIn: 10, TokensOut: 2},
			{ID: "two", Agent: "beta", Project: "p", UpdatedAt: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), TokensIn: 20, TokensOut: 3},
		},
		analyticsPivotConfig: analytics.PivotConfig{Rows: []analytics.Dimension{analytics.DimensionAgent}, Columns: []analytics.Dimension{analytics.DimensionTimeline}, Values: []analytics.Metric{analytics.MetricSessions}},
	}
	view := m.View()
	for _, want := range []string{"row", "alpha", "beta", "2026-05-01", "Total", "sessions:2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("pivot view missing %q:\n%s", want, view)
		}
	}
}

func TestPivotNavigationClampsAtOriginAndMovesGrid(t *testing.T) {
	cfg := config.Default()
	m := Model{mode: modeAnalytics, cfg: cfg, keys: resolveKeys(cfg.Keybinds), analyticsPivotRowCursor: 0, analyticsPivotColCursor: 0}
	updated, _ := m.updateAnalytics("k")
	m = updated.(Model)
	updated, _ = m.updateAnalytics("h")
	m = updated.(Model)
	if m.analyticsPivotRowCursor != 0 || m.analyticsPivotColCursor != 0 {
		t.Fatalf("navigation moved before origin: row=%d col=%d", m.analyticsPivotRowCursor, m.analyticsPivotColCursor)
	}
	updated, _ = m.updateAnalytics("j")
	m = updated.(Model)
	updated, _ = m.updateAnalytics("l")
	m = updated.(Model)
	if m.analyticsPivotRowCursor != 1 || m.analyticsPivotColCursor != 1 {
		t.Fatalf("navigation = row %d col %d, want 1,1", m.analyticsPivotRowCursor, m.analyticsPivotColCursor)
	}
}
