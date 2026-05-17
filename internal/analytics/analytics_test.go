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
