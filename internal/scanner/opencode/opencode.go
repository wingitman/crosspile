package opencode

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
	_ "modernc.org/sqlite"
)

type sessionRow struct {
	id               string
	title            string
	directory        string
	agent            sql.NullString
	modelJSON        sql.NullString
	timeCreated      int64
	timeUpdated      int64
	cost             float64
	tokensIn         int64
	tokensOut        int64
	tokensReasoning  int64
	tokensCacheRead  int64
	tokensCacheWrite int64
}

type messageRow struct {
	id          string
	sessionID   string
	timeCreated int64
	data        string
}

type partRow struct {
	messageID   string
	timeCreated int64
	data        string
}

type messageData struct {
	Role       string `json:"role"`
	Agent      string `json:"agent"`
	Mode       string `json:"mode"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
	Time       struct {
		Created   int64 `json:"created"`
		Completed int64 `json:"completed"`
	} `json:"time"`
}

type partData struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Tool  string          `json:"tool"`
	Title string          `json:"title"`
	State json.RawMessage `json:"state"`
	File  string          `json:"file"`
}

type modelData struct {
	ID         string `json:"id"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant"`
}

func Scan(ctx context.Context, locations []string) ([]model.Session, []string) {
	var path string
	for _, candidate := range dbPaths() {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return nil, nil
	}

	sessions, err := scanDB(ctx, path, locations)
	if err != nil {
		return nil, []string{fmt.Sprintf("opencode: %v", err)}
	}
	return sessions, nil
}

func scanDB(ctx context.Context, path string, locations []string) ([]model.Session, error) {
	dsn := sqliteURI(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `select id, title, directory, agent, model, time_created, time_updated, cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write from session order by time_updated desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessionRows []sessionRow
	for rows.Next() {
		var r sessionRow
		if err := rows.Scan(&r.id, &r.title, &r.directory, &r.agent, &r.modelJSON, &r.timeCreated, &r.timeUpdated, &r.cost, &r.tokensIn, &r.tokensOut, &r.tokensReasoning, &r.tokensCacheRead, &r.tokensCacheWrite); err != nil {
			return nil, err
		}
		if inLocations(r.directory, locations) {
			sessionRows = append(sessionRows, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(sessionRows) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(sessionRows))
	for _, r := range sessionRows {
		ids = append(ids, r.id)
	}
	messages, err := loadMessages(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	parts, err := loadParts(ctx, db, ids)
	if err != nil {
		return nil, err
	}

	out := make([]model.Session, 0, len(sessionRows))
	for _, r := range sessionRows {
		s := model.Session{
			ID:               r.id,
			Title:            r.title,
			Agent:            "opencode",
			Directory:        r.directory,
			Project:          filepath.Base(r.directory),
			CreatedAt:        unixish(r.timeCreated),
			UpdatedAt:        unixish(r.timeUpdated),
			Cost:             r.cost,
			TokensIn:         r.tokensIn,
			TokensOut:        r.tokensOut,
			TokensReasoning:  r.tokensReasoning,
			TokensCacheRead:  r.tokensCacheRead,
			TokensCacheWrite: r.tokensCacheWrite,
			Source:           path,
			SourceKind:       "sqlite",
			Context:          r.directory,
			RawRefs:          map[string][]string{"session": []string{r.id}},
			Metadata:         map[string]string{"source_table": "session"},
		}
		if r.agent.Valid && r.agent.String != "" {
			s.Mode = r.agent.String
		}
		if r.modelJSON.Valid {
			var md modelData
			if json.Unmarshal([]byte(r.modelJSON.String), &md) == nil {
				s.Model = firstNonEmpty(md.ID, md.ModelID)
				s.Provider = md.ProviderID
				if md.Variant != "" && s.Model != "" {
					s.Model += " (" + md.Variant + ")"
				}
			}
		}

		for _, mr := range messages[r.id] {
			msg := model.Message{ID: mr.id, CreatedAt: unixish(mr.timeCreated)}
			var md messageData
			if json.Unmarshal([]byte(mr.data), &md) == nil {
				msg.Role = md.Role
				if s.Mode == "" {
					s.Mode = firstNonEmpty(md.Agent, md.Mode)
				}
				if s.Model == "" {
					s.Model = md.ModelID
				}
				if s.Provider == "" {
					s.Provider = md.ProviderID
				}
				if md.Time.Created > 0 {
					msg.CreatedAt = unixish(md.Time.Created)
				}
			}
			if msg.Role == "" {
				msg.Role = "unknown"
			}
			for _, pr := range parts[mr.id] {
				part := parsePart(pr.data)
				if part.Type == "tool" && part.Meta != "" {
					s.Tools = appendUnique(s.Tools, part.Meta)
				}
				if part.Type == "file" && part.Meta != "" {
					s.Files = appendUnique(s.Files, part.Meta)
				}
				if part.Text != "" || part.Meta != "" {
					msg.Parts = append(msg.Parts, part)
				}
			}
			if len(msg.Parts) > 0 {
				s.Messages = append(s.Messages, msg)
				s.RawRefs["message"] = append(s.RawRefs["message"], mr.id)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func loadMessages(ctx context.Context, db *sql.DB, sessionIDs []string) (map[string][]messageRow, error) {
	out := map[string][]messageRow{}
	for _, batch := range batches(sessionIDs, 200) {
		query := `select id, session_id, time_created, data from message where session_id in (` + placeholders(len(batch)) + `) order by session_id, time_created, id`
		args := make([]any, len(batch))
		for i := range batch {
			args[i] = batch[i]
		}
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r messageRow
			if err := rows.Scan(&r.id, &r.sessionID, &r.timeCreated, &r.data); err != nil {
				rows.Close()
				return nil, err
			}
			out[r.sessionID] = append(out[r.sessionID], r)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func loadParts(ctx context.Context, db *sql.DB, sessionIDs []string) (map[string][]partRow, error) {
	out := map[string][]partRow{}
	for _, batch := range batches(sessionIDs, 200) {
		query := `select message_id, time_created, data from part where session_id in (` + placeholders(len(batch)) + `) order by message_id, time_created, id`
		args := make([]any, len(batch))
		for i := range batch {
			args[i] = batch[i]
		}
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r partRow
			if err := rows.Scan(&r.messageID, &r.timeCreated, &r.data); err != nil {
				rows.Close()
				return nil, err
			}
			out[r.messageID] = append(out[r.messageID], r)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func parsePart(raw string) model.Part {
	var pd partData
	if json.Unmarshal([]byte(raw), &pd) != nil {
		return model.Part{Type: "raw", Text: raw}
	}
	part := model.Part{Type: pd.Type}
	switch pd.Type {
	case "text":
		part.Text = pd.Text
	case "tool":
		part.Meta = pd.Tool
		if part.Meta == "" {
			part.Meta = pd.Title
		}
		part.Text = summarizeTool(pd)
	case "file":
		part.Meta = pd.File
		part.Text = pd.Text
	case "patch":
		part.Text = pd.Text
	case "reasoning", "step-start", "step-finish":
		part.Text = pd.Text
	default:
		part.Text = pd.Text
		part.Meta = firstNonEmpty(pd.Tool, pd.Title, pd.File)
	}
	return part
}

func summarizeTool(pd partData) string {
	tool := firstNonEmpty(pd.Tool, pd.Title)
	if tool == "" {
		tool = "tool"
	}
	if len(pd.State) == 0 {
		return "[" + tool + "]"
	}
	var state struct {
		Status string         `json:"status"`
		Input  map[string]any `json:"input"`
	}
	if json.Unmarshal(pd.State, &state) != nil {
		return "[" + tool + "]"
	}
	bits := []string{"[" + tool}
	if state.Status != "" {
		bits = append(bits, state.Status)
	}
	for _, k := range []string{"filePath", "path", "pattern", "command", "description"} {
		if v, ok := state.Input[k]; ok {
			bits = append(bits, fmt.Sprint(v))
			break
		}
	}
	return strings.Join(bits, " ") + "]"
}

func dbPaths() []string {
	var paths []string
	if override := os.Getenv("OPENCODE_DATA_DIR"); override != "" {
		paths = append(paths, filepath.Join(override, "opencode.db"))
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "opencode", "opencode.db"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return paths
	}
	switch runtime.GOOS {
	case "darwin":
		paths = append(paths, filepath.Join(home, "Library", "Application Support", "opencode", "opencode.db"))
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			paths = append(paths, filepath.Join(local, "opencode", "opencode.db"))
		}
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			paths = append(paths, filepath.Join(appdata, "opencode", "opencode.db"))
		}
	default:
		paths = append(paths, filepath.Join(home, ".local", "share", "opencode", "opencode.db"))
	}
	paths = append(paths, filepath.Join(home, ".local", "share", "opencode", "opencode.db"))
	return paths
}

func sqliteURI(path string) string {
	p := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file:" + p + "?mode=ro&_pragma=busy_timeout(5000)"
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

func unixish(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	switch {
	case v > 1_000_000_000_000:
		return time.UnixMilli(v)
	case v > 1_000_000_000:
		return time.Unix(v, 0)
	default:
		return time.UnixMilli(v)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func appendUnique(vals []string, next string) []string {
	if next == "" {
		return vals
	}
	for _, v := range vals {
		if v == next {
			return vals
		}
	}
	return append(vals, next)
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func batches(vals []string, size int) [][]string {
	if size <= 0 {
		size = 100
	}
	var out [][]string
	for len(vals) > 0 {
		n := size
		if len(vals) < n {
			n = len(vals)
		}
		out = append(out, vals[:n])
		vals = vals[n:]
	}
	return out
}

func sortMessages(msgs []model.Message) {
	sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].CreatedAt.Before(msgs[j].CreatedAt) })
}
