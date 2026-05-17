package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
)

type event struct {
	UUID      string          `json:"uuid"`
	SessionID string          `json:"sessionId"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	CWD       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
	Text      string          `json:"text"`
}

type messagePayload struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func Scan(ctx context.Context, locations []string) ([]model.Session, []string) {
	files := candidateFiles()
	var out []model.Session
	var warnings []string
	for _, file := range files {
		if ctx.Err() != nil {
			return out, warnings
		}
		s, err := parseFile(file)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("claude: %s: %v", file, err))
			continue
		}
		if s.ID == "" || !inLocations(s.Directory, locations) {
			continue
		}
		out = append(out, s)
	}
	return out, warnings
}

func candidateFiles() []string {
	seen := map[string]bool{}
	var roots []string
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		roots = append(roots, filepath.Join(dir, "projects"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".claude", "projects"))
	}
	var files []string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			if !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}

func parseFile(path string) (model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Session{}, err
	}
	defer f.Close()

	s := model.Session{Agent: "claude", Source: path, SourceKind: "jsonl", ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.SessionID != "" {
			s.ID = ev.SessionID
		}
		if ev.CWD != "" {
			s.Directory = ev.CWD
			s.Project = filepath.Base(ev.CWD)
			s.Context = ev.CWD
		}
		at := parseTime(ev.Timestamp)
		if s.CreatedAt.IsZero() || (!at.IsZero() && at.Before(s.CreatedAt)) {
			s.CreatedAt = at
		}
		if at.After(s.UpdatedAt) {
			s.UpdatedAt = at
		}

		msg := eventMessage(ev, at)
		if len(msg.Parts) > 0 {
			s.Messages = append(s.Messages, msg)
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return s, err
	}
	if s.Directory == "" {
		s.Directory = decodeProjectPath(filepath.Base(filepath.Dir(path)))
		s.Project = filepath.Base(s.Directory)
	}
	if s.Title == "" {
		s.Title = firstLine(s.PromptText())
	}
	if s.Title == "" {
		s.Title = "Claude session " + s.ID
	}
	return s, nil
}

func eventMessage(ev event, at time.Time) model.Message {
	msg := model.Message{ID: ev.UUID, CreatedAt: at}
	if len(ev.Message) > 0 {
		var mp messagePayload
		if json.Unmarshal(ev.Message, &mp) == nil {
			msg.Role = mp.Role
			text := contentText(mp.Content)
			if text != "" {
				msg.Parts = append(msg.Parts, model.Part{Type: "text", Text: text})
			}
		}
	}
	if msg.Role == "" {
		switch ev.Type {
		case "user", "assistant", "system":
			msg.Role = ev.Type
		default:
			msg.Role = "event"
		}
	}
	if len(msg.Parts) == 0 && ev.Text != "" {
		msg.Parts = append(msg.Parts, model.Part{Type: "text", Text: ev.Text})
	}
	return msg
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			} else if b.Name != "" {
				parts = append(parts, "["+b.Type+" "+b.Name+"]")
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func decodeProjectPath(s string) string {
	if strings.Contains(s, "-") && !strings.Contains(s, string(filepath.Separator)) {
		s = strings.TrimPrefix(s, "-")
		return string(filepath.Separator) + strings.ReplaceAll(s, "-", string(filepath.Separator))
	}
	return s
}

func inLocations(path string, locations []string) bool {
	if len(locations) == 0 {
		return true
	}
	path = clean(path)
	for _, loc := range locations {
		loc = clean(loc)
		if path == loc {
			return true
		}
		rel, err := filepath.Rel(loc, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
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
