package analytics

import (
	"testing"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

func TestBuildTimelineByDay(t *testing.T) {
	sessions := []model.Session{
		{ID: "one", UpdatedAt: time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC), TokensIn: 10, TokensOut: 5, Cost: 0.10},
		{ID: "two", UpdatedAt: time.Date(2026, 3, 4, 11, 0, 0, 0, time.UTC), TokensIn: 20, TokensOut: 5, Cost: 0.20},
		{ID: "three", UpdatedAt: time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC), TokensIn: 5, TokensOut: 5, Cost: 0.05},
	}
	rows := Build(sessions, DimensionTimeline, BucketDay)
	if len(rows) != 2 {
		t.Fatalf("rows=%#v", rows)
	}
	if rows[0].Key != "2026-03-04" || rows[0].Sessions != 2 || rows[0].Tokens != 40 {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
}

func TestBuildByAgent(t *testing.T) {
	sessions := []model.Session{
		{Agent: "opencode", TokensIn: 10, Cost: 0.1},
		{Agent: "claude", TokensIn: 30, Cost: 0.3},
		{Agent: "opencode", TokensOut: 5, Cost: 0.2},
	}
	rows := Build(sessions, DimensionAgent, BucketAll)
	if rows[0].Key != "opencode" || rows[0].Sessions != 2 {
		t.Fatalf("expected highest cost first, got %#v", rows)
	}
	if Value(rows[1], MetricCostPerRequest) != "$0.3000" {
		t.Fatalf("bad cost/request: %s", Value(rows[0], MetricCostPerRequest))
	}
}

func TestBuildPivotMultiAxisAndRatios(t *testing.T) {
	sessions := []model.Session{
		{Agent: "a", Mode: "chat", Tools: []string{"git", "go"}, UpdatedAt: time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), TokensIn: 10, TokensOut: 10, Cost: 0.20},
		{Agent: "a", Mode: "code", Tools: []string{"go"}, UpdatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), TokensIn: 30, Cost: 0.30},
		{Agent: "b", Mode: "chat", Skills: []string{"testing"}, UpdatedAt: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), TokensOut: 10, Cost: 0.10},
	}
	table := BuildPivot(sessions, PivotConfig{
		Rows:        []Dimension{DimensionAgent, DimensionMode},
		Columns:     []Dimension{DimensionTimeline},
		Values:      []Metric{MetricSessions, MetricCostPerRequest},
		Granularity: BucketDay,
	})
	if got, want := len(table.Rows), 3; got != want {
		t.Fatalf("row headers=%d, want %d", got, want)
	}
	if got, want := len(table.Columns), 2; got != want {
		t.Fatalf("column headers=%d, want %d", got, want)
	}
	if table.Rows[0].Values[0] != "a" || table.Rows[0].Values[1] != "chat" {
		t.Fatalf("unexpected first row header: %#v", table.Rows[0])
	}
	if got := table.Cells[0][0].Sessions; got != 1 {
		t.Fatalf("chat/day cell sessions=%d, want 1", got)
	}
	if got := table.Cells[0][0].Values[MetricCostPerRequest]; got != 0.2 {
		t.Fatalf("cost/request=%v, want .2", got)
	}
	if got := table.GrandTotal.Sessions; got != 3 {
		t.Fatalf("grand total sessions=%d, want 3", got)
	}
}

func TestBuildPivotMultiValuesAndFilter(t *testing.T) {
	sessions := []model.Session{
		{Agent: "keep", Tools: []string{"git", "go"}, Skills: []string{"review"}, Cost: 1},
		{Agent: "skip", Tools: []string{"git"}, Cost: 2},
	}
	table := BuildPivot(sessions, PivotConfig{
		Filters: []Filter{{Dimension: DimensionTool, Values: []string{"go"}}},
		Rows:    []Dimension{DimensionTool},
		Columns: []Dimension{DimensionSkill},
		Values:  []Metric{MetricCost},
	})
	if len(table.Rows) != 2 || table.Rows[0].Values[0] != "git" || table.Rows[1].Values[0] != "go" {
		t.Fatalf("unexpected tool headers: %#v", table.Rows)
	}
	if len(table.Columns) != 1 || table.Columns[0].Values[0] != "review" {
		t.Fatalf("unexpected skill headers: %#v", table.Columns)
	}
	if table.GrandTotal.Cost != 1 {
		t.Fatalf("filtered grand total=%v, want 1", table.GrandTotal.Cost)
	}
}
