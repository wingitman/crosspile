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
