package search

import (
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

type Query struct {
	Raw      string
	Free     []string
	Agent    string
	Project  string
	Session  string
	Model    string
	Question string
	Answer   string
	From     time.Time
	To       time.Time
}

func Parse(raw string) Query {
	q := Query{Raw: raw}
	for _, tok := range strings.Fields(raw) {
		key, val, ok := strings.Cut(tok, ":")
		if !ok || val == "" {
			q.Free = append(q.Free, tok)
			continue
		}
		key = strings.ToLower(key)
		switch key {
		case "agent":
			q.Agent = val
		case "project", "proj":
			q.Project = val
		case "sid", "session", "id":
			q.Session = val
		case "model":
			q.Model = val
		case "q", "question", "prompt":
			q.Question = val
		case "a", "answer", "response":
			q.Answer = val
		case "from":
			q.From = parseDate(val)
		case "to":
			q.To = parseDate(val)
		default:
			q.Free = append(q.Free, tok)
		}
	}
	return q
}

func Filter(sessions []model.Session, raw string) []model.Session {
	q := Parse(raw)
	if strings.TrimSpace(raw) == "" {
		return append([]model.Session(nil), sessions...)
	}
	out := make([]model.Session, 0, len(sessions))
	for _, s := range sessions {
		if Match(s, q) {
			out = append(out, s)
		}
	}
	return out
}

func Match(s model.Session, q Query) bool {
	if q.Agent != "" && !contains(s.Agent, q.Agent) && !contains(s.Mode, q.Agent) {
		return false
	}
	if q.Project != "" && !contains(s.ProjectName(), q.Project) && !contains(s.Directory, q.Project) {
		return false
	}
	if q.Session != "" && !contains(s.ID, q.Session) {
		return false
	}
	if q.Model != "" && !contains(s.Model, q.Model) && !contains(s.Provider, q.Model) {
		return false
	}
	if q.Question != "" && !contains(s.PromptText(), q.Question) {
		return false
	}
	if q.Answer != "" && !contains(s.ResponseText(), q.Answer) {
		return false
	}
	if !q.From.IsZero() && s.UpdatedAt.Before(q.From) {
		return false
	}
	if !q.To.IsZero() && s.UpdatedAt.After(q.To.Add(24*time.Hour)) {
		return false
	}
	text := s.AllText()
	for _, free := range q.Free {
		if !contains(text, free) {
			return false
		}
	}
	return true
}

func contains(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func parseDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}
