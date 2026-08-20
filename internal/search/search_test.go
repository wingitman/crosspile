package search

import (
	"strings"
	"testing"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

func TestFilterStructuredFields(t *testing.T) {
	sessions := []model.Session{
		{
			ID:        "ses_one",
			Title:     "Fix login bug",
			Agent:     "opencode",
			Model:     "gpt-5.5",
			Project:   "app",
			UpdatedAt: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
			Messages:  []model.Message{{Role: "user", Parts: []model.Part{{Type: "text", Text: "please fix auth"}}}},
		},
		{
			ID:        "ses_two",
			Title:     "Write docs",
			Agent:     "claude",
			Project:   "docs",
			UpdatedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		},
	}

	got := Filter(sessions, "agent:opencode project:app from:2026-05-01 q:auth")
	if len(got) != 1 || got[0].ID != "ses_one" {
		t.Fatalf("expected ses_one, got %#v", got)
	}
}

func TestFilterFreeText(t *testing.T) {
	sessions := []model.Session{{ID: "ses_one", Title: "Central AI history", Agent: "opencode"}}
	got := Filter(sessions, "central history")
	if len(got) != 1 {
		t.Fatalf("expected free-text match")
	}
}

func TestFilterExpandedMetadata(t *testing.T) {
	sessions := []model.Session{{
		ID: "ses_one", Agent: "opencode", Mode: "build", Project: "crosspile", LocationName: "Work",
		Model: "gpt-5.5", Provider: "opencode", Tools: []string{"bash"}, Skills: []string{"omarchy"},
		TokensIn: 9000, TokensOut: 2000, Cost: 0.15,
		UpdatedAt: time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		Messages:  []model.Message{{Role: "user", Parts: []model.Part{{Type: "text", Text: "build a TUI"}}}},
	}}

	got := Filter(sessions, `agent:open project:cross location:work model:gpt tool:bash skill:omarchy tokens:>10000 cost:<0.20 q:"build a TUI" updated:2026-05-01..2026-05-17`)
	if len(got) != 1 {
		t.Fatalf("expected expanded metadata match, got %d", len(got))
	}
}

func TestFilterNegation(t *testing.T) {
	sessions := []model.Session{{ID: "one", Agent: "opencode"}, {ID: "two", Agent: "claude"}}
	got := Filter(sessions, `-agent:claude`)
	if len(got) != 1 || got[0].ID != "one" {
		t.Fatalf("expected only opencode session, got %#v", got)
	}
}

func TestFilterMonthDayFromAndTo(t *testing.T) {
	nowYear := time.Now().Year()
	sessions := []model.Session{
		{ID: "before", UpdatedAt: time.Date(nowYear, 3, 3, 23, 0, 0, 0, time.Local)},
		{ID: "on", UpdatedAt: time.Date(nowYear, 3, 4, 12, 0, 0, 0, time.Local)},
		{ID: "after", UpdatedAt: time.Date(nowYear, 3, 5, 1, 0, 0, 0, time.Local)},
	}
	got := Filter(sessions, "from:Mar04")
	if len(got) != 2 || got[0].ID != "on" || got[1].ID != "after" {
		t.Fatalf("from:Mar04 got %#v", got)
	}
	got = Filter(sessions, "date:Mar04")
	if len(got) != 1 || got[0].ID != "on" {
		t.Fatalf("date:Mar04 got %#v", got)
	}
}

func TestFilterISODateInclusive(t *testing.T) {
	sessions := []model.Session{
		{ID: "before", UpdatedAt: time.Date(2026, 3, 3, 23, 0, 0, 0, time.Local)},
		{ID: "on", UpdatedAt: time.Date(2026, 3, 4, 23, 59, 0, 0, time.Local)},
		{ID: "after", UpdatedAt: time.Date(2026, 3, 5, 1, 0, 0, 0, time.Local)},
	}
	got := Filter(sessions, "from:2026-03-04 to:2026-03-04")
	if len(got) != 1 || got[0].ID != "on" {
		t.Fatalf("ISO inclusive range got %#v", got)
	}
}

func TestSummaryDescribesParsedFilters(t *testing.T) {
	summary := strings.Join(Summary("from:Mar04 agent:opencode tokens:>100"), " ")
	for _, want := range []string{"updated >=", "agent:opencode", "tokens >= 100"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q in %q", want, summary)
		}
	}
}

func TestFilterHealthAndIssues(t *testing.T) {
	sessions := []model.Session{{ID: "bad", Health: "corrupted", Issues: []string{"invalid JSON"}}, {ID: "ok", Health: "healthy"}}
	got := Filter(sessions, "health:corrupt issue:json")
	if len(got) != 1 || got[0].ID != "bad" {
		t.Fatalf("unexpected health filter result: %#v", got)
	}
}
