package generic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

type record struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Text      string `json:"text"`
	Prompt    string `json:"prompt"`
	Response  string `json:"response"`
	CreatedAt string `json:"created_at"`
	Timestamp string `json:"timestamp"`
	Agent     string `json:"agent"`
	Model     string `json:"model"`
	Title     string `json:"title"`
	CWD       string `json:"cwd"`
}

func Scan(ctx context.Context, locations []string) ([]model.Session, []string) {
	var out []model.Session
	var warnings []string
	for _, root := range locations {
		if ctx.Err() != nil {
			return out, warnings
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if !looksGeneric(path) {
				return nil
			}
			s, err := parse(path)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("generic: %s: %v", path, err))
				return nil
			}
			if len(s.Messages) > 0 {
				out = append(out, s)
			}
			return nil
		})
	}
	return out, warnings
}

func looksGeneric(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	if !(strings.Contains(lower, "/.clawbot/") || strings.Contains(lower, "/clawbot/")) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jsonl" || ext == ".json"
}

func parse(path string) (model.Session, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jsonl" {
		return parseJSONL(path)
	}
	return parseJSON(path)
}

func parseJSONL(path string) (model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Session{}, err
	}
	defer f.Close()
	s := baseSession(path)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 2*1024*1024)
	for scanner.Scan() {
		addRecord(&s, scanner.Bytes())
	}
	return finalize(s), scanner.Err()
}

func parseJSON(path string) (model.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Session{}, err
	}
	s := baseSession(path)
	var records []json.RawMessage
	if json.Unmarshal(data, &records) == nil {
		for _, raw := range records {
			addRecord(&s, raw)
		}
		return finalize(s), nil
	}
	addRecord(&s, data)
	return finalize(s), nil
}

func baseSession(path string) model.Session {
	dir := filepath.Dir(path)
	return model.Session{ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Agent: "generic", Project: filepath.Base(dir), Directory: dir, Source: path}
}

func addRecord(s *model.Session, raw []byte) {
	var r record
	if json.Unmarshal(raw, &r) != nil {
		return
	}
	if r.SessionID != "" {
		s.ID = r.SessionID
	} else if r.ID != "" && s.ID == "" {
		s.ID = r.ID
	}
	if r.Agent != "" {
		s.Agent = r.Agent
	}
	if r.Model != "" {
		s.Model = r.Model
	}
	if r.Title != "" {
		s.Title = r.Title
	}
	if r.CWD != "" {
		s.Directory = r.CWD
		s.Project = filepath.Base(r.CWD)
	}
	at := parseTime(firstNonEmpty(r.Timestamp, r.CreatedAt))
	if s.CreatedAt.IsZero() || (!at.IsZero() && at.Before(s.CreatedAt)) {
		s.CreatedAt = at
	}
	if at.After(s.UpdatedAt) {
		s.UpdatedAt = at
	}
	if r.Prompt != "" {
		s.Messages = append(s.Messages, model.Message{ID: r.ID + "-prompt", Role: "user", CreatedAt: at, Parts: []model.Part{{Type: "text", Text: r.Prompt}}})
	}
	if r.Response != "" {
		s.Messages = append(s.Messages, model.Message{ID: r.ID + "-response", Role: "assistant", CreatedAt: at, Parts: []model.Part{{Type: "text", Text: r.Response}}})
	}
	text := firstNonEmpty(r.Content, r.Text)
	if text != "" {
		role := r.Role
		if role == "" {
			role = "unknown"
		}
		s.Messages = append(s.Messages, model.Message{ID: r.ID, Role: role, CreatedAt: at, Parts: []model.Part{{Type: "text", Text: text}}})
	}
}

func finalize(s model.Session) model.Session {
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(s.Source), filepath.Ext(s.Source))
	}
	if s.Title == "" {
		s.Title = firstLine(s.PromptText())
	}
	if s.Title == "" {
		s.Title = "Generic AI session " + s.ID
	}
	if s.UpdatedAt.IsZero() {
		if info, err := os.Stat(s.Source); err == nil {
			s.UpdatedAt = info.ModTime()
		}
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = s.UpdatedAt
	}
	return s
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	line := strings.SplitN(s, "\n", 2)[0]
	if len(line) > 90 {
		line = line[:89] + "..."
	}
	return line
}
