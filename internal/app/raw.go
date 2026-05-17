package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/crosspile/internal/model"
	_ "modernc.org/sqlite"
)

type rawData struct {
	Tables []rawTable
}

type rawTable struct {
	Name    string
	Columns []string
	Rows    [][]string
}

func loadRawDataCmd(s model.Session) tea.Cmd {
	return func() tea.Msg {
		data, err := loadRawData(context.Background(), s)
		return rawLoadedMsg{data: data, err: err}
	}
}

func loadRawData(ctx context.Context, s model.Session) (rawData, error) {
	switch s.SourceKind {
	case "sqlite":
		return loadSQLiteRaw(ctx, s)
	case "json", "jsonl":
		return rawData{Tables: []rawTable{{Name: "source", Columns: []string{"field", "value"}, Rows: [][]string{{"source", s.Source}, {"kind", s.SourceKind}, {"session", s.ID}}}}}, nil
	default:
		return rawData{}, fmt.Errorf("no raw view for source kind %q", s.SourceKind)
	}
}

func loadSQLiteRaw(ctx context.Context, s model.Session) (rawData, error) {
	db, err := sql.Open("sqlite", sqliteURIForApp(s.Source))
	if err != nil {
		return rawData{}, err
	}
	defer db.Close()
	tables := []rawTable{}
	for _, spec := range []struct {
		name, query string
		args        []any
	}{
		{"session", `select * from session where id = ?`, []any{s.ID}},
		{"message", `select * from message where session_id = ? order by time_created, id`, []any{s.ID}},
		{"part", `select * from part where session_id = ? order by time_created, id`, []any{s.ID}},
		{"todo", `select * from todo where session_id = ? order by id`, []any{s.ID}},
	} {
		t, err := queryRawTable(ctx, db, spec.name, spec.query, spec.args...)
		if err == nil && len(t.Rows) > 0 {
			tables = append(tables, t)
		}
	}
	if len(tables) == 0 {
		return rawData{}, fmt.Errorf("no raw rows found for session %s", s.ID)
	}
	return rawData{Tables: tables}, nil
}

func queryRawTable(ctx context.Context, db *sql.DB, name, query string, args ...any) (rawTable, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return rawTable{}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return rawTable{}, err
	}
	out := rawTable{Name: name, Columns: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return out, err
		}
		row := make([]string, len(cols))
		for i, val := range vals {
			row[i] = rawValueString(val)
		}
		out.Rows = append(out.Rows, row)
	}
	return out, rows.Err()
}

func rawValueString(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case []byte:
		return prettyJSON(string(v))
	default:
		return prettyJSON(fmt.Sprint(v))
	}
}

func prettyJSON(s string) string {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(b)
		}
	}
	return s
}

func sqliteURIForApp(path string) string {
	p := strings.ReplaceAll(path, `\`, `/`)
	if !strings.HasPrefix(p, "/") && len(p) > 1 && p[1] == ':' {
		p = "/" + p
	}
	return "file:" + p + "?mode=ro&_pragma=busy_timeout(5000)"
}

func (m Model) updateRaw(key string) (tea.Model, tea.Cmd) {
	if len(m.raw.Tables) == 0 {
		m.mode = modeNormal
		return m, nil
	}
	t := m.raw.Tables[m.rawTable]
	switch key {
	case m.keys.back, m.keys.clearSearch:
		m.mode = modeNormal
	case m.keys.rawNextTable:
		m.rawTable = (m.rawTable + 1) % len(m.raw.Tables)
		m.rawRow, m.rawCol, m.rawRowOffset, m.rawColOffset = 0, 0, 0, 0
	case m.keys.rawPrevTable:
		m.rawTable--
		if m.rawTable < 0 {
			m.rawTable = len(m.raw.Tables) - 1
		}
		m.rawRow, m.rawCol, m.rawRowOffset, m.rawColOffset = 0, 0, 0, 0
	case m.keys.up:
		m.rawRow--
	case m.keys.down:
		m.rawRow++
	case m.keys.left:
		m.rawCol--
	case m.keys.right:
		m.rawCol++
	case m.keys.pageUp:
		m.rawRow -= 10
	case m.keys.pageDown:
		m.rawRow += 10
	}
	if m.rawRow < 0 {
		m.rawRow = 0
	}
	if m.rawRow >= len(t.Rows) {
		m.rawRow = len(t.Rows) - 1
	}
	if m.rawCol < 0 {
		m.rawCol = 0
	}
	if m.rawCol >= len(t.Columns) {
		m.rawCol = len(t.Columns) - 1
	}
	if m.rawRow < m.rawRowOffset {
		m.rawRowOffset = m.rawRow
	}
	if m.rawRow >= m.rawRowOffset+m.rawVisibleRows() {
		m.rawRowOffset = m.rawRow - m.rawVisibleRows() + 1
	}
	if m.rawCol < m.rawColOffset {
		m.rawColOffset = m.rawCol
	}
	if m.rawCol >= m.rawColOffset+m.rawVisibleCols() {
		m.rawColOffset = m.rawCol - m.rawVisibleCols() + 1
	}
	return m, nil
}

func (m Model) rawVisibleRows() int { return max(3, m.height-13) }
func (m Model) rawVisibleCols() int { return max(1, (m.width-4)/22) }
