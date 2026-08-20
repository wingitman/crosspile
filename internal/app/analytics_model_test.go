package app

import (
	"testing"

	"github.com/wingitman/crosspile/internal/analytics"
	"github.com/wingitman/crosspile/internal/config"
)

func TestAnalyticsMetricNavigationIncludesUncheckedMetrics(t *testing.T) {
	cfg := config.Default()
	m := Model{
		cfg:                 cfg,
		keys:                resolveKeys(cfg.Keybinds),
		analyticsConfigOpen: true,
		analyticsFocus:      1,
		analyticsMetrics:    []bool{true, false, false, false, false, false, false, false, false},
	}

	updated, _ := m.updateAnalytics(m.keys.down)
	m = updated.(Model)
	if m.analyticsMetricCursor != int(analytics.MetricTokens) {
		t.Fatalf("cursor = %d, want unchecked tokens metric", m.analyticsMetricCursor)
	}

	updated, _ = m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if !m.analyticsMetrics[int(analytics.MetricTokens)] {
		t.Fatal("Enter did not select the unchecked metric")
	}
}

func TestAnalyticsMetricCanBeDeselectedAndReselected(t *testing.T) {
	cfg := config.Default()
	m := Model{
		cfg:                 cfg,
		keys:                resolveKeys(cfg.Keybinds),
		analyticsConfigOpen: true,
		analyticsFocus:      1,
		analyticsMetrics:    []bool{true, false, false, false, false, false, false, false, false},
	}

	updated, _ := m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if m.analyticsMetrics[0] {
		t.Fatal("Enter did not deselect the current metric")
	}
	if m.analyticsMetricCursor != 0 {
		t.Fatalf("cursor moved after deselection: %d", m.analyticsMetricCursor)
	}

	updated, _ = m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if !m.analyticsMetrics[0] {
		t.Fatal("Enter did not reselect the current metric")
	}
}

func TestAnalyticsEnterAppliesGroupSelection(t *testing.T) {
	cfg := config.Default()
	m := Model{
		cfg:                 cfg,
		keys:                resolveKeys(cfg.Keybinds),
		analyticsConfigOpen: true,
		analyticsFocus:      0,
	}

	updated, _ := m.updateAnalytics(m.keys.confirm)
	got := updated.(Model)
	if got.analyticsConfigOpen {
		t.Fatal("Enter did not close analytics configuration for group selection")
	}
}

func TestPivotConfigurationAddsFieldsToFocusedWell(t *testing.T) {
	cfg := config.Default()
	m := Model{cfg: cfg, keys: resolveKeys(cfg.Keybinds), analyticsConfigOpen: true, analyticsMetrics: analytics.DefaultSelectedMetrics(), analyticsPivotConfig: analytics.PivotConfig{Rows: []analytics.Dimension{analytics.DimensionProject}}}
	updated, _ := m.updateAnalytics(m.keys.down)
	m = updated.(Model)
	updated, _ = m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if !m.analyticsPlacementOpen {
		t.Fatal("Enter did not open placement picker")
	}
	updated, _ = m.updateAnalytics("j")
	m = updated.(Model)
	updated, _ = m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if len(m.analyticsPivotConfig.Columns) != 1 || m.analyticsPivotConfig.Columns[0] != analytics.DimensionAgent {
		t.Fatalf("columns = %#v, want agent", m.analyticsPivotConfig.Columns)
	}
}

func TestPivotConfigurationRemovesFieldWithEnter(t *testing.T) {
	cfg := config.Default()
	m := Model{cfg: cfg, keys: resolveKeys(cfg.Keybinds), analyticsConfigOpen: true, analyticsMetrics: analytics.DefaultSelectedMetrics(), analyticsPivotConfig: analytics.PivotConfig{Rows: []analytics.Dimension{analytics.DimensionAgent}}}
	for i, dimension := range analytics.Dimensions {
		if dimension == analytics.DimensionAgent {
			m.analyticsPivotCursor = i
			break
		}
	}
	updated, _ := m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if len(m.analyticsPivotConfig.Rows) != 0 {
		t.Fatalf("rows = %#v, want removal", m.analyticsPivotConfig.Rows)
	}
}

func TestPivotConfigurationTraversesSectionsBothDirections(t *testing.T) {
	cfg := config.Default()
	m := Model{cfg: cfg, keys: resolveKeys(cfg.Keybinds), analyticsConfigOpen: true}
	for want := 1; want < 2; want++ {
		updated, _ := m.updateAnalytics("tab")
		m = updated.(Model)
		if m.analyticsPivotFocus != want {
			t.Fatalf("tab focus = %d, want %d", m.analyticsPivotFocus, want)
		}
	}
	updated, _ := m.updateAnalytics("shift+tab")
	m = updated.(Model)
	if m.analyticsPivotFocus != 0 {
		t.Fatalf("shift-tab focus = %d, want dimensions section", m.analyticsPivotFocus)
	}
}

func TestPivotConfigurationTogglesMetrics(t *testing.T) {
	cfg := config.Default()
	m := Model{cfg: cfg, keys: resolveKeys(cfg.Keybinds), analyticsConfigOpen: true, analyticsPivotConfig: analytics.PivotConfig{Rows: []analytics.Dimension{analytics.DimensionProject}}}
	updated, _ := m.updateAnalytics("tab")
	m = updated.(Model)
	updated, _ = m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if len(m.analyticsPivotConfig.Values) != 1 || m.analyticsPivotConfig.Values[0] != analytics.MetricSessions {
		t.Fatalf("values = %#v, want sessions", m.analyticsPivotConfig.Values)
	}
	updated, _ = m.updateAnalytics(m.keys.confirm)
	m = updated.(Model)
	if len(m.analyticsPivotConfig.Values) != 0 {
		t.Fatalf("values = %#v, want empty after toggle", m.analyticsPivotConfig.Values)
	}
}
