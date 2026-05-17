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
