package app

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/crosspile/internal/model"
	_ "modernc.org/sqlite"
)

const (
	rawPageSize     = 200
	rawPreviewChars = 50
	rawMaxCellSize  = 128 * 1024
)

type rawData struct {
	Tables []rawTable
}

type rawTable struct {
	Name     string
	Columns  []string
	Rows     [][]string
	Total    int
	Page     int
	PageSize int
}

type rawTableSpec struct {
	name       string
	query      string
	countQuery string
	args       []any
}

func loadRawDataCmd(s model.Session) tea.Cmd {
	return func() tea.Msg {
		data, err := loadRawData(context.Background(), s)
		return rawLoadedMsg{session: s, data: data, err: err}
	}
}

func loadRawTablePageCmd(s model.Session, tableName string, page int) tea.Cmd {
	return func() tea.Msg {
		table, err := loadRawTablePage(context.Background(), s, tableName, page)
		return rawPageLoadedMsg{sessionID: s.ID, source: s.Source, table: tableName, data: table, err: err}
	}
}

func loadRawData(ctx context.Context, s model.Session) (rawData, error) {
	switch s.SourceKind {
	case "sqlite":
		return loadSQLiteRaw(ctx, s)
	case "json", "jsonl":
		rows := [][]string{{"source", s.Source}, {"kind", s.SourceKind}, {"session", s.ID}}
		return rawData{Tables: []rawTable{{Name: "source", Columns: []string{"field", "value"}, Rows: rows, Total: len(rows), PageSize: rawPageSize}}}, nil
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
	for _, spec := range rawTableSpecs(s) {
		t, err := queryRawTablePage(ctx, db, spec, 0, rawPageSize)
		if err == nil && t.Total > 0 {
			tables = append(tables, t)
		}
	}
	if len(tables) == 0 {
		return rawData{}, fmt.Errorf("no raw rows found for session %s", s.ID)
	}
	return rawData{Tables: tables}, nil
}

func rawTableSpecs(s model.Session) []rawTableSpec {
	return []rawTableSpec{
		{"session", `select * from session where id = ?`, `select count(*) from session where id = ?`, []any{s.ID}},
		{"message", `select * from message where session_id = ? order by time_created, id`, `select count(*) from message where session_id = ?`, []any{s.ID}},
		{"part", `select * from part where session_id = ? order by time_created, id`, `select count(*) from part where session_id = ?`, []any{s.ID}},
		{"todo", `select * from todo where session_id = ? order by id`, `select count(*) from todo where session_id = ?`, []any{s.ID}},
	}
}

func loadRawTablePage(ctx context.Context, s model.Session, tableName string, page int) (rawTable, error) {
	if s.SourceKind != "sqlite" {
		return rawTable{}, fmt.Errorf("raw paging is not available for source kind %q", s.SourceKind)
	}
	db, err := sql.Open("sqlite", sqliteURIForApp(s.Source))
	if err != nil {
		return rawTable{}, err
	}
	defer db.Close()
	for _, spec := range rawTableSpecs(s) {
		if spec.name == tableName {
			return queryRawTablePage(ctx, db, spec, page, rawPageSize)
		}
	}
	return rawTable{}, fmt.Errorf("unknown raw table %q", tableName)
}

func queryRawTablePage(ctx context.Context, db *sql.DB, spec rawTableSpec, page, pageSize int) (rawTable, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = rawPageSize
	}
	var total int
	if err := db.QueryRowContext(ctx, spec.countQuery, spec.args...).Scan(&total); err != nil {
		return rawTable{}, err
	}
	if total > 0 {
		lastPage := int(math.Ceil(float64(total)/float64(pageSize))) - 1
		if page > lastPage {
			page = lastPage
		}
	}
	args := append([]any{}, spec.args...)
	args = append(args, pageSize, page*pageSize)
	rows, err := db.QueryContext(ctx, spec.query+` limit ? offset ?`, args...)
	if err != nil {
		return rawTable{}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return rawTable{}, err
	}
	out := rawTable{Name: spec.name, Columns: cols, Total: total, Page: page, PageSize: pageSize}
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

func loadRawCell(ctx context.Context, s model.Session, tableName string, page, pageSize, rowIndex, colIndex int) (string, error) {
	if s.SourceKind != "sqlite" {
		return "", fmt.Errorf("raw cell loading is not available for source kind %q", s.SourceKind)
	}
	db, err := sql.Open("sqlite", sqliteURIForApp(s.Source))
	if err != nil {
		return "", err
	}
	defer db.Close()
	for _, spec := range rawTableSpecs(s) {
		if spec.name == tableName {
			return queryRawCell(ctx, db, spec, page, pageSize, rowIndex, colIndex)
		}
	}
	return "", fmt.Errorf("unknown raw table %q", tableName)
}

func queryRawCell(ctx context.Context, db *sql.DB, spec rawTableSpec, page, pageSize, rowIndex, colIndex int) (string, error) {
	if pageSize <= 0 {
		pageSize = rawPageSize
	}
	offset := page*pageSize + rowIndex
	if offset < 0 {
		offset = 0
	}
	args := append([]any{}, spec.args...)
	args = append(args, 1, offset)
	rows, err := db.QueryContext(ctx, spec.query+` limit ? offset ?`, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if colIndex < 0 || colIndex >= len(cols) {
		return "", fmt.Errorf("raw column %d is out of range", colIndex)
	}
	if !rows.Next() {
		return "", fmt.Errorf("raw row %d is out of range", rowIndex)
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return rawFullValueString(vals[colIndex]), nil
}

func rawValueString(v any) string {
	s := rawFullValueString(v)
	if len(s) > rawMaxCellSize {
		return s[:rawMaxCellSize] + fmt.Sprintf("\n... (%d bytes truncated)", len(s)-rawMaxCellSize)
	}
	return s
}

func rawFullValueString(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func rawCellPreview(s string) string {
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= rawPreviewChars {
			break
		}
		switch r {
		case '\r', '\n', '\t':
			r = ' '
		}
		b.WriteRune(r)
		count++
	}
	preview := b.String()
	return strings.Join(strings.Fields(preview), " ")
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
		m.rawLoading = false
		m.mode = modeNormal
	case m.keys.quit:
		return m, tea.Quit
	case m.keys.confirm:
		m.rawRowOpen = !m.rawRowOpen
	case m.keys.openDocument:
		m.status = "opening raw cell..."
		return m, m.prepareRawCellEditorCmd()
	case m.keys.export:
		m.exportPrompt = true
		m.exportTarget = exportTargetRaw
		m.exportChoice = 0
		m.status = "choose export format"
	case m.keys.rawNextTable:
		m.rawLoading = false
		m.rawTable = (m.rawTable + 1) % len(m.raw.Tables)
		m.rawRow, m.rawCol, m.rawRowOffset, m.rawColOffset = 0, 0, 0, 0
		m.rawRowOpen = false
	case m.keys.rawPrevTable:
		m.rawLoading = false
		m.rawTable--
		if m.rawTable < 0 {
			m.rawTable = len(m.raw.Tables) - 1
		}
		m.rawRow, m.rawCol, m.rawRowOffset, m.rawColOffset = 0, 0, 0, 0
		m.rawRowOpen = false
	case m.keys.up:
		m.rawRow--
		m.rawRowOpen = false
	case m.keys.down:
		m.rawRow++
		m.rawRowOpen = false
	case m.keys.left:
		m.rawCol--
	case m.keys.right:
		m.rawCol++
	case m.keys.pageUp:
		if m.rawLoading {
			return m, nil
		}
		if t.Page > 0 {
			m.rawLoading = true
			m.status = "loading raw page..."
			return m, loadRawTablePageCmd(m.rawSession, t.Name, t.Page-1)
		}
	case m.keys.pageDown:
		if m.rawLoading {
			return m, nil
		}
		if t.Page+1 < t.rawPageCount() {
			m.rawLoading = true
			m.status = "loading raw page..."
			return m, loadRawTablePageCmd(m.rawSession, t.Name, t.Page+1)
		}
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

func (t rawTable) rawPageCount() int {
	if t.Total <= 0 || t.PageSize <= 0 {
		return 1
	}
	return (t.Total + t.PageSize - 1) / t.PageSize
}
