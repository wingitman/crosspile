package codewhale

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

// CodeWhale stores one JSON document per session. The decoder intentionally
// accepts the common field names used by both exported and local sessions.
func Scan(ctx context.Context, locations []string) ([]model.Session, []string) {
	var sessions []model.Session
	var warnings []string
	for _, path := range candidates() {
		if ctx.Err() != nil {
			break
		}
		s, err := parse(path)
		if err != nil && s.ID == "" {
			warnings = append(warnings, fmt.Sprintf("codewhale: %s: %v", path, err))
			continue
		}
		if s.ID != "" && (inLocations(s.Directory, locations) || !s.Healthy()) {
			sessions = append(sessions, s)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("codewhale: %s: %v", path, err))
		}
	}
	return sessions, warnings
}

func candidates() []string {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".codewhale", "sessions")
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".json") {
			out = append(out, path)
		}
		return nil
	})
	return out
}

type record struct {
	ID, SessionID, Role, Text, Prompt, Response, Agent, Model, Title, CWD, Mode, Timestamp, CreatedAt string
	Content                                                                                           json.RawMessage   `json:"content"`
	Messages                                                                                          []json.RawMessage `json:"messages"`
	Metadata                                                                                          json.RawMessage   `json:"metadata"`
}

type metadata struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Model     string `json:"model"`
	Workspace string `json:"workspace"`
	Mode      string `json:"mode"`
}

func parse(path string) (model.Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.Session{}, err
	}
	var records []record
	if json.Unmarshal(b, &records) != nil {
		var r record
		if err := json.Unmarshal(b, &r); err != nil {
			return model.Session{ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Agent: "codewhale", Source: path, Directory: filepath.Dir(path), Health: "corrupted", Issues: []string{"invalid JSON"}}, err
		}
		applyMetadata(&r)
		records = []record{r}
	}
	s := model.Session{Agent: "codewhale", Source: path, SourceKind: "json", Health: "healthy"}
	for _, r := range records {
		applyMetadata(&r)
		add(&s, r)
		for _, raw := range r.Messages {
			var nested record
			if json.Unmarshal(raw, &nested) == nil {
				add(&s, nested)
			} else {
				s.Health = "degraded"
				s.Issues = appendUnique(s.Issues, "invalid message JSON")
			}
		}
	}
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if s.Directory == "" {
		s.Directory = filepath.Dir(path)
	}
	s.Project, s.Context = filepath.Base(s.Directory), s.Directory
	if s.Title == "" {
		s.Title = firstLine(s.PromptText())
	}
	if s.Title == "" {
		s.Title = "CodeWhale session " + s.ID
	}
	if s.UpdatedAt.IsZero() {
		if info, e := os.Stat(path); e == nil {
			s.UpdatedAt = info.ModTime()
		}
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = s.UpdatedAt
	}
	return s, nil
}

func applyMetadata(r *record) {
	if len(r.Metadata) == 0 {
		return
	}
	var m metadata
	if json.Unmarshal(r.Metadata, &m) != nil {
		return
	}
	r.ID = firstNonEmpty(r.ID, m.ID)
	r.Title = firstNonEmpty(r.Title, m.Title)
	r.CreatedAt = firstNonEmpty(r.CreatedAt, m.CreatedAt)
	r.Timestamp = firstNonEmpty(r.Timestamp, m.UpdatedAt)
	r.Model = firstNonEmpty(r.Model, m.Model)
	r.Mode = firstNonEmpty(r.Mode, m.Mode)
	r.Agent = firstNonEmpty(r.Agent, "codewhale")
	r.CWD = firstNonEmpty(r.CWD, m.Workspace)
}

func add(s *model.Session, r record) {
	if r.SessionID != "" {
		s.ID = r.SessionID
	}
	if r.ID != "" && s.ID == "" {
		s.ID = r.ID
	}
	if r.Agent != "" {
		s.Agent = r.Agent
	}
	s.Model = firstNonEmpty(s.Model, r.Model)
	s.Mode = firstNonEmpty(s.Mode, r.Mode)
	s.Title = firstNonEmpty(s.Title, r.Title)
	if r.CWD != "" {
		s.Directory = r.CWD
	}
	at := parseTime(firstNonEmpty(r.Timestamp, r.CreatedAt))
	if s.CreatedAt.IsZero() || (!at.IsZero() && at.Before(s.CreatedAt)) {
		s.CreatedAt = at
	}
	if at.After(s.UpdatedAt) {
		s.UpdatedAt = at
	}
	for _, pair := range []struct{ role, text string }{{"user", firstNonEmpty(r.Prompt, contentText(r.Content))}, {"assistant", firstNonEmpty(r.Response, r.Text)}} {
		if pair.text != "" {
			s.Messages = append(s.Messages, model.Message{Role: pair.role, CreatedAt: at, Parts: []model.Part{{Type: "text", Text: pair.text}}})
		}
	}
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []struct {
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
		Content  string `json:"content"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var out []string
	for _, block := range blocks {
		if block.Text != "" {
			out = append(out, block.Text)
		} else if block.Thinking != "" {
			out = append(out, block.Thinking)
		} else if block.Content != "" {
			out = append(out, block.Content)
		}
	}
	return strings.Join(out, "\n")
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func inLocations(path string, locations []string) bool {
	if len(locations) == 0 {
		return true
	}
	a, _ := filepath.Abs(path)
	for _, loc := range locations {
		l, _ := filepath.Abs(loc)
		rel, e := filepath.Rel(l, a)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
func parseTime(v string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, e := time.Parse(layout, v); e == nil {
			return t
		}
	}
	return time.Time{}
}
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 90 {
		s = s[:89] + "..."
	}
	return s
}
