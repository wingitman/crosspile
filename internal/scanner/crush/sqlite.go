package crush

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/model"
	_ "modernc.org/sqlite"
)

func parseSQLite(ctx context.Context, path, workspace string) ([]model.Session, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	sessionCols, err := tableColumns(ctx, db, "sessions")
	if err != nil {
		return []model.Session{degradedDB(path, workspace, "unsupported database: sessions table unavailable")}, err
	}
	if !sessionCols["id"] {
		return []model.Session{degradedDB(path, workspace, "unsupported database: sessions.id column unavailable")}, fmt.Errorf("sessions table has no id column")
	}
	rows, err := queryDynamic(ctx, db, "sessions", sessionCols, "updated_at")
	if err != nil {
		return []model.Session{degradedDB(path, workspace, "unable to read sessions")}, err
	}
	defer rows.Close()
	var sessions []model.Session
	for rows.Next() {
		values, err := scanValues(rows)
		if err != nil {
			return sessions, err
		}
		s := model.Session{ID: text(values["id"]), Title: text(values["title"]), Agent: "crush", Source: path, SourceKind: "sqlite", CreatedAt: crushTime(values["created_at"]), UpdatedAt: crushTime(values["updated_at"]), TokensIn: int64Value(values["prompt_tokens"]), TokensOut: int64Value(values["completion_tokens"]), Cost: floatValue(values["cost"]), Health: "healthy", RawRefs: map[string][]string{"database": {path}}, Metadata: map[string]string{"source_table": "sessions"}}
		if workspace != "" {
			s.Directory, s.Context, s.Project = workspace, workspace, filepath.Base(workspace)
		} else {
			s.Directory, s.Context, s.Project = filepath.Dir(path), filepath.Dir(path), filepath.Base(filepath.Dir(path))
		}
		if s.Title == "" {
			s.Title = "Crush session " + s.ID
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return sessions, err
	}
	if len(sessions) == 0 {
		return []model.Session{degradedDB(path, workspace, "database contains no sessions")}, nil
	}

	messageCols, err := tableColumns(ctx, db, "messages")
	if err != nil {
		markDegraded(sessions, "messages table unavailable")
		return sessions, nil
	}
	if !messageCols["session_id"] || !messageCols["parts"] {
		markDegraded(sessions, "messages table lacks session_id or parts")
		return sessions, nil
	}
	mrows, err := queryDynamic(ctx, db, "messages", messageCols, "created_at")
	if err != nil {
		markDegraded(sessions, "unable to read messages")
		return sessions, nil
	}
	defer mrows.Close()
	for mrows.Next() {
		values, err := scanValues(mrows)
		if err != nil {
			markDegraded(sessions, "invalid message row")
			continue
		}
		var s *model.Session
		for i := range sessions {
			if sessions[i].ID == text(values["session_id"]) {
				s = &sessions[i]
				break
			}
		}
		if s == nil {
			continue
		}
		msg := model.Message{ID: text(values["id"]), Role: text(values["role"]), CreatedAt: crushTime(values["created_at"])}
		if msg.Role == "" {
			msg.Role = "unknown"
		}
		if s.Model == "" {
			s.Model = text(values["model"])
		}
		if s.Provider == "" {
			s.Provider = text(values["provider"])
		}
		parts, issue := crushParts(text(values["parts"]))
		if issue != "" {
			s.Health = "degraded"
			s.Issues = appendUnique(s.Issues, issue)
		}
		for _, part := range parts {
			if part.Type == "tool" && part.Meta != "" {
				s.Tools = appendUnique(s.Tools, part.Meta)
			}
			if part.Type == "file" && part.Meta != "" {
				s.Files = appendUnique(s.Files, part.Meta)
			}
		}
		if len(parts) > 0 {
			msg.Parts = parts
			s.Messages = append(s.Messages, msg)
			s.RawRefs["message"] = append(s.RawRefs["message"], msg.ID)
		}
	}
	if err := mrows.Err(); err != nil {
		markDegraded(sessions, "message iteration failed")
	}
	return sessions, nil
}

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var d any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &d, &pk); err != nil {
			return nil, err
		}
		cols[strings.ToLower(name)] = true
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %s missing", table)
	}
	return cols, rows.Err()
}

func queryDynamic(ctx context.Context, db *sql.DB, table string, cols map[string]bool, order string) (*sql.Rows, error) {
	selected := []string{}
	for _, col := range []string{"id", "session_id", "role", "parts", "model", "provider", "title", "prompt_tokens", "completion_tokens", "cost", "created_at", "updated_at"} {
		if cols[col] {
			selected = append(selected, col)
		}
	}
	query := `SELECT ` + strings.Join(selected, ", ") + ` FROM ` + table
	if cols[order] {
		query += ` ORDER BY ` + order
	}
	return db.QueryContext(ctx, query)
}

func scanValues(rows *sql.Rows) (map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	raw := make([]any, len(columns))
	dest := make([]any, len(columns))
	for i := range raw {
		dest[i] = &raw[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for i, col := range columns {
		out[strings.ToLower(col)] = raw[i]
	}
	return out, nil
}
func markDegraded(s []model.Session, issue string) {
	for i := range s {
		s[i].Health = "degraded"
		s[i].Issues = appendUnique(s[i].Issues, issue)
	}
}

func crushParts(raw string) ([]model.Part, string) {
	var values []map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		if strings.TrimSpace(raw) == "" {
			return nil, "empty message parts"
		}
		return []model.Part{{Type: "raw", Text: raw}}, "invalid message parts JSON"
	}
	var out []model.Part
	for _, value := range values {
		typ := text(value["type"])
		if typ == "" {
			typ = "text"
		}
		p := model.Part{Type: typ, Text: text(value["text"]), Meta: firstNonEmpty(text(value["tool"]), text(value["name"]), text(value["path"]), text(value["file"]))}
		if p.Type == "tool" && p.Text == "" {
			p.Text = "[" + firstNonEmpty(p.Meta, "tool") + "]"
		}
		if p.Text != "" || p.Meta != "" {
			out = append(out, p)
		}
	}
	return out, ""
}

func projectDatabases(roots []string) map[string]string {
	out := map[string]string{}
	for _, root := range roots {
		data, err := os.ReadFile(filepath.Join(root, "projects.json"))
		if err != nil {
			continue
		}
		var raw any
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		walkProjects(raw, func(path, dataDir string) {
			if dataDir == "" {
				dataDir = ".crush"
			}
			if !filepath.IsAbs(dataDir) {
				dataDir = filepath.Join(path, dataDir)
			}
			out[clean(filepath.Join(dataDir, "crush.db"))] = path
		})
	}
	return out
}
func walkProjects(v any, add func(string, string)) {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			walkProjects(item, add)
		}
	case map[string]any:
		if path := text(x["path"]); path != "" {
			add(path, text(x["data_dir"]))
		}
		for _, item := range x {
			if _, ok := item.(map[string]any); ok {
				walkProjects(item, add)
			}
		}
	}
}
func degradedDB(path, workspace, issue string) model.Session {
	dir := workspace
	if dir == "" {
		dir = filepath.Dir(path)
	}
	return model.Session{ID: "crush-db-" + strconv.FormatInt(time.Now().UnixNano(), 10), Title: "Crush database", Agent: "crush", Directory: dir, Source: path, SourceKind: "sqlite", Health: "corrupted", Issues: []string{issue}}
}
func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}
func int64Value(v any) int64   { n, _ := strconv.ParseInt(text(v), 10, 64); return n }
func floatValue(v any) float64 { n, _ := strconv.ParseFloat(text(v), 64); return n }
func crushTime(v any) time.Time {
	n := int64Value(v)
	if n == 0 {
		return time.Time{}
	}
	if n < 1e11 {
		return time.Unix(n, 0)
	}
	return time.UnixMilli(n)
}
