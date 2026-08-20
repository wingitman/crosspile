package crush

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

func Scan(ctx context.Context, locations []string) ([]model.Session, []string) {
	paths, roots := candidates()
	var out []model.Session
	var warnings []string
	for _, candidate := range paths {
		if ctx.Err() != nil {
			break
		}
		var sessions []model.Session
		var err error
		if strings.EqualFold(filepath.Base(candidate.path), "crush.db") {
			sessions, err = parseSQLite(ctx, candidate.path, candidate.workspace)
		} else {
			var s model.Session
			s, err = parseJSON(candidate.path)
			if s.ID != "" {
				sessions = []model.Session{s}
			}
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("crush: %s: %v", candidate.path, err))
		}
		for _, s := range sessions {
			if s.ID != "" && (inLocations(s.Directory, locations) || !s.Healthy()) {
				out = append(out, s)
			}
		}
	}
	if len(paths) == 0 && anyRootExists(roots) {
		warnings = append(warnings, "crush: no readable session files found (unavailable)")
	}
	return out, warnings
}

func anyRootExists(roots []string) bool {
	for _, root := range roots {
		if _, err := os.Stat(root); err == nil {
			return true
		}
	}
	return false
}

type candidate struct {
	path      string
	workspace string
}

func candidates() ([]candidate, []string) {
	home, _ := os.UserHomeDir()
	var roots []string
	if v := os.Getenv("CRUSH_DATA_DIR"); v != "" {
		roots = append(roots, v)
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		roots = append(roots, filepath.Join(v, "crush"))
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".local", "share", "crush"), filepath.Join(home, ".config", "crush"))
	}
	seen := map[string]bool{}
	workspaceByDB := projectDatabases(roots)
	var files []candidate
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err == nil && d != nil && !d.IsDir() && filepath.Base(path) != "projects.json" && (strings.EqualFold(filepath.Base(path), "crush.db") || strings.EqualFold(filepath.Ext(path), ".json") || strings.EqualFold(filepath.Ext(path), ".jsonl")) && !seen[path] {
				seen[path] = true
				files = append(files, candidate{path: path, workspace: workspaceByDB[clean(path)]})
			}
			return nil
		})
	}
	for path, workspace := range workspaceByDB {
		if !seen[path] {
			seen[path] = true
			files = append(files, candidate{path: path, workspace: workspace})
		}
	}
	return files, roots
}

type record struct{ ID, SessionID, Role, Content, Text, Prompt, Response, Agent, Model, Title, CWD, Timestamp, CreatedAt string }

func parseJSON(path string) (model.Session, error) {
	s := model.Session{ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Agent: "crush", Source: path, SourceKind: "json", Health: "healthy"}
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024), 4*1024*1024)
	for sc.Scan() {
		var r record
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			s.Health = "corrupted"
			s.Issues = append(s.Issues, "invalid JSON record")
			continue
		}
		add(&s, r)
	}
	if err := sc.Err(); err != nil {
		return s, err
	}
	if s.Directory == "" {
		s.Directory = filepath.Dir(path)
	}
	s.Project, s.Context = filepath.Base(s.Directory), s.Directory
	if s.Title == "" {
		s.Title = "Crush session " + s.ID
	}
	if s.UpdatedAt.IsZero() {
		if i, e := os.Stat(path); e == nil {
			s.UpdatedAt = i.ModTime()
		}
	}
	s.CreatedAt = firstTime(s.CreatedAt, s.UpdatedAt)
	if len(s.Issues) > 0 {
		s.Health = "degraded"
	}
	return s, nil
}
func add(s *model.Session, r record) {
	if r.SessionID != "" {
		s.ID = r.SessionID
	}
	if r.ID != "" && s.ID == "" {
		s.ID = r.ID
	}
	s.Model = firstNonEmpty(s.Model, r.Model)
	s.Title = firstNonEmpty(s.Title, r.Title)
	if r.CWD != "" {
		s.Directory = r.CWD
	}
	at := parseTime(firstNonEmpty(r.Timestamp, r.CreatedAt))
	s.CreatedAt = firstTime(s.CreatedAt, at)
	if at.After(s.UpdatedAt) {
		s.UpdatedAt = at
	}
	for _, v := range []struct{ role, text string }{{"user", firstNonEmpty(r.Prompt, r.Content)}, {"assistant", firstNonEmpty(r.Response, r.Text)}} {
		if v.text != "" {
			s.Messages = append(s.Messages, model.Message{Role: v.role, CreatedAt: at, Parts: []model.Part{{Type: "text", Text: v.text}}})
		}
	}
}
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
func firstTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	return a
}
func parseTime(v string) time.Time {
	for _, l := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, e := time.Parse(l, v); e == nil {
			return t
		}
	}
	return time.Time{}
}
func inLocations(path string, locations []string) bool {
	if len(locations) == 0 {
		return true
	}
	a, _ := filepath.Abs(path)
	for _, loc := range locations {
		l, _ := filepath.Abs(loc)
		r, e := filepath.Rel(l, a)
		if e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func clean(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
