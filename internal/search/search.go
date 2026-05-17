package search

import (
	"strconv"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

type Query struct {
	Raw         string
	Free        []string
	NotFree     []string
	Include     map[string][]string
	Exclude     map[string][]string
	From        time.Time
	To          time.Time
	CreatedFrom time.Time
	CreatedTo   time.Time
	UpdatedFrom time.Time
	UpdatedTo   time.Time
	MinTokens   *int64
	MaxTokens   *int64
	MinCost     *float64
	MaxCost     *float64
}

func Parse(raw string) Query {
	q := Query{Raw: raw, Include: map[string][]string{}, Exclude: map[string][]string{}}
	for _, tok := range splitTokens(raw) {
		neg := false
		if strings.HasPrefix(tok, "-") || strings.HasPrefix(tok, "!") {
			neg = true
			tok = strings.TrimPrefix(strings.TrimPrefix(tok, "-"), "!")
		}
		key, val, ok := strings.Cut(tok, ":")
		if !ok || val == "" {
			if neg {
				q.NotFree = append(q.NotFree, tok)
			} else {
				q.Free = append(q.Free, tok)
			}
			continue
		}
		key = normalizeKey(key)
		val = strings.Trim(val, `"'`)
		switch key {
		case "from":
			q.From = parseDateValue(val, false)
		case "to":
			q.To = parseDateValue(val, true)
		case "date", "created", "updated":
			from, to := parseRange(val)
			if key == "created" {
				q.CreatedFrom, q.CreatedTo = from, to
			} else if key == "updated" {
				q.UpdatedFrom, q.UpdatedTo = from, to
			} else {
				q.From, q.To = from, to
			}
		case "tokens":
			q.MinTokens, q.MaxTokens = parseIntComparison(val)
		case "cost":
			q.MinCost, q.MaxCost = parseFloatComparison(val)
		default:
			if neg {
				q.Exclude[key] = append(q.Exclude[key], val)
			} else {
				q.Include[key] = append(q.Include[key], val)
			}
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
	if !q.From.IsZero() && s.UpdatedAt.Before(q.From) {
		return false
	}
	if !q.To.IsZero() && s.UpdatedAt.After(q.To) {
		return false
	}
	if !q.CreatedFrom.IsZero() && s.CreatedAt.Before(q.CreatedFrom) {
		return false
	}
	if !q.CreatedTo.IsZero() && s.CreatedAt.After(q.CreatedTo) {
		return false
	}
	if !q.UpdatedFrom.IsZero() && s.UpdatedAt.Before(q.UpdatedFrom) {
		return false
	}
	if !q.UpdatedTo.IsZero() && s.UpdatedAt.After(q.UpdatedTo) {
		return false
	}
	if q.MinTokens != nil && s.TotalTokens() < *q.MinTokens {
		return false
	}
	if q.MaxTokens != nil && s.TotalTokens() > *q.MaxTokens {
		return false
	}
	if q.MinCost != nil && s.Cost < *q.MinCost {
		return false
	}
	if q.MaxCost != nil && s.Cost > *q.MaxCost {
		return false
	}
	for key, vals := range q.Include {
		if !matchAny(fieldValues(s, key), vals) {
			return false
		}
	}
	for key, vals := range q.Exclude {
		if matchAny(fieldValues(s, key), vals) {
			return false
		}
	}
	all := s.AllText()
	for _, free := range q.Free {
		if !contains(all, free) {
			return false
		}
	}
	for _, free := range q.NotFree {
		if contains(all, free) {
			return false
		}
	}
	return true
}

func fieldValues(s model.Session, key string) []string {
	switch key {
	case "agent":
		return []string{s.Agent, s.Mode}
	case "mode":
		return []string{s.Mode}
	case "context", "ctx":
		return []string{s.Context, s.Directory}
	case "location", "loc":
		return []string{s.LocationName, s.LocationPath}
	case "project", "proj":
		return []string{s.ProjectName(), s.Directory}
	case "session", "sid", "id":
		return []string{s.ID}
	case "model":
		return []string{s.Model}
	case "provider":
		return []string{s.Provider}
	case "tool", "tools":
		return s.Tools
	case "skill", "skills":
		return s.Skills
	case "file", "files":
		return s.Files
	case "source":
		return []string{s.SourceKind, s.Source}
	case "q", "question", "prompt":
		return []string{s.PromptText()}
	case "a", "answer", "response":
		return []string{s.ResponseText()}
	case "todo", "todos":
		vals := make([]string, 0, len(s.Todos))
		for _, t := range s.Todos {
			vals = append(vals, t.Content, t.Status, t.Priority)
		}
		return vals
	case "role":
		vals := make([]string, 0, len(s.Messages))
		for _, msg := range s.Messages {
			vals = append(vals, msg.Role)
		}
		return vals
	default:
		if v, ok := s.Metadata[key]; ok {
			return []string{v}
		}
		return []string{s.AllText()}
	}
}

func matchAny(fields []string, needles []string) bool {
	for _, needle := range needles {
		for _, field := range fields {
			if contains(field, needle) {
				return true
			}
		}
	}
	return false
}

func contains(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func normalizeKey(k string) string { return strings.ToLower(strings.TrimSpace(k)) }

func splitTokens(s string) []string {
	var out []string
	var b strings.Builder
	quote := rune(0)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func parseRange(s string) (time.Time, time.Time) {
	if strings.Contains(s, "..") {
		a, b, _ := strings.Cut(s, "..")
		return parseDateValue(a, false), parseDateValue(b, true)
	}
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err == nil {
			return time.Now().AddDate(0, 0, -days), time.Now()
		}
	}
	t := parseDateValue(s, false)
	return t, endOfDay(t)
}

func parseDateValue(s string, end bool) time.Time {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}
	}
	now := time.Now()
	switch s {
	case "today":
		base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		if end {
			return endOfDay(base)
		}
		return base
	case "yesterday":
		base := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.Local)
		if end {
			return endOfDay(base)
		}
		return base
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			if end && layout == "2006-01-02" {
				return endOfDay(t)
			}
			return t
		}
	}
	return time.Time{}
}

func endOfDay(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func parseIntComparison(s string) (*int64, *int64) {
	if strings.HasPrefix(s, ">") {
		v := parseInt(strings.TrimPrefix(s, ">"))
		return v, nil
	}
	if strings.HasPrefix(s, "<") {
		v := parseInt(strings.TrimPrefix(s, "<"))
		return nil, v
	}
	v := parseInt(s)
	return v, v
}

func parseFloatComparison(s string) (*float64, *float64) {
	if strings.HasPrefix(s, ">") {
		v := parseFloat(strings.TrimPrefix(s, ">"))
		return v, nil
	}
	if strings.HasPrefix(s, "<") {
		v := parseFloat(strings.TrimPrefix(s, "<"))
		return nil, v
	}
	v := parseFloat(s)
	return v, v
}

func parseInt(s string) *int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return nil
	}
	return &v
}
func parseFloat(s string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &v
}
