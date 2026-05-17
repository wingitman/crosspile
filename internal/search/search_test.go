package search

import (
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
