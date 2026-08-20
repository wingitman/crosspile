package app

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/crosspile/internal/analytics"
	"github.com/wingitman/crosspile/internal/model"
)

func (m Model) exportRawTableCmd(format string) tea.Cmd {
	if len(m.raw.Tables) == 0 || m.rawTable < 0 || m.rawTable >= len(m.raw.Tables) {
		return func() tea.Msg { return statusMsg("no raw table selected") }
	}
	session := m.rawSession
	table := m.raw.Tables[m.rawTable]
	exportDir := m.cfg.Output.ExportDir
	return func() tea.Msg {
		rows, err := exportRows(context.Background(), session, table)
		if err != nil {
			return errorMsg(err.Error())
		}
		path, err := writeExport(exportDir, session.ID, table.Name, format, table.Columns, rows)
		if err != nil {
			return errorMsg(err.Error())
		}
		return exportedStatus(path, revealExportFile(path))
	}
}

func (m Model) exportAnalyticsCmd(format string) tea.Cmd {
	config := m.pivotConfig()
	table := analytics.BuildPivot(m.filtered, config)
	dimension := analytics.DimensionTimeline
	if len(config.Rows) > 0 {
		dimension = config.Rows[0]
	}
	bucket := config.Period
	exportDir := m.cfg.Output.ExportDir
	return func() tea.Msg {
		columns, data := pivotAnalyticsExportData(table)
		path, err := writeAnalyticsExport(exportDir, dimension, bucket, format, columns, data)
		if err != nil {
			return errorMsg(err.Error())
		}
		return exportedStatus(path, revealExportFile(path))
	}
}

func pivotAnalyticsExportData(table analytics.PivotTable) ([]string, [][]string) {
	metrics := table.Config.Values
	if len(metrics) == 0 {
		metrics = []analytics.Metric{analytics.MetricSessions}
	}
	columns := []string{"row"}
	for _, header := range table.Columns {
		for _, metric := range metrics {
			label := header.Key
			if label == "" {
				label = "(all)"
			}
			columns = append(columns, label+"/"+metric.String())
		}
	}
	if len(table.Columns) == 0 {
		for _, metric := range metrics {
			columns = append(columns, "(all)/"+metric.String())
		}
	}
	rows := make([][]string, 0, len(table.Rows)+1)
	for r, header := range table.Rows {
		row := []string{header.Key}
		for c := range table.Columns {
			for _, metric := range metrics {
				row = append(row, analytics.Value(table.Cells[r][c].Row, metric))
			}
		}
		rows = append(rows, row)
	}
	grand := []string{"grand total"}
	for _, metric := range metrics {
		grand = append(grand, analytics.Value(table.GrandTotal, metric))
	}
	rows = append(rows, grand)
	return columns, rows
}

func analyticsExportData(dimension analytics.Dimension, metrics []analytics.Metric, rows []analytics.Row, total analytics.Row) ([]string, [][]string) {
	columns := []string{dimension.String()}
	for _, metric := range metrics {
		columns = append(columns, metric.String())
	}
	out := make([][]string, 0, len(rows)+1)
	for _, row := range rows {
		out = append(out, analyticsExportRow(row, metrics))
	}
	total.Key = "total"
	out = append(out, analyticsExportRow(total, metrics))
	return columns, out
}

func analyticsExportRow(row analytics.Row, metrics []analytics.Metric) []string {
	out := []string{row.Key}
	for _, metric := range metrics {
		out = append(out, analytics.Value(row, metric))
	}
	return out
}

func writeAnalyticsExport(exportDir string, dimension analytics.Dimension, bucket analytics.Bucket, format string, columns []string, rows [][]string) (string, error) {
	dir := resolveExportDir(exportDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	name := safeFileName(fmt.Sprintf("crosspile-analytics-%s-%s-%s.%s", dimension, bucket, stamp, format))
	path := filepath.Join(dir, name)
	switch format {
	case "csv":
		return path, writeCSV(path, columns, rows)
	case "txt":
		return path, writeText(path, columns, rows)
	default:
		return "", fmt.Errorf("unsupported analytics export format %q", format)
	}
}

func exportRows(ctx context.Context, session model.Session, table rawTable) ([][]string, error) {
	if session.SourceKind == "sqlite" {
		db, err := sql.Open("sqlite", sqliteURIForApp(session.Source))
		if err != nil {
			return nil, err
		}
		defer db.Close()
		for _, spec := range rawTableSpecs(session) {
			if spec.name == table.Name {
				return queryRawTableAll(ctx, db, spec)
			}
		}
		return nil, fmt.Errorf("unknown raw table %q", table.Name)
	}
	return table.Rows, nil
}

func queryRawTableAll(ctx context.Context, db *sql.DB, spec rawTableSpec) ([][]string, error) {
	rows, err := db.QueryContext(ctx, spec.query, spec.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out [][]string
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
			row[i] = rawFullValueString(val)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func writeExport(exportDir, sessionID, tableName, format string, columns []string, rows [][]string) (string, error) {
	dir := resolveExportDir(exportDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	name := safeFileName(fmt.Sprintf("crosspile-%s-%s-%s.%s", sessionID, tableName, stamp, format))
	path := filepath.Join(dir, name)
	switch format {
	case "csv":
		return path, writeCSV(path, columns, rows)
	case "json":
		return path, writeJSON(path, columns, rows)
	case "txt":
		return path, writeText(path, columns, rows)
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
	}
}

func exportedStatus(path string, revealErr error) tea.Msg {
	if revealErr != nil {
		return statusMsg("exported " + path + "; file explorer failed: " + revealErr.Error())
	}
	return statusMsg("exported " + path + " and opened file explorer")
}

func revealExportFile(path string) error {
	cmd := fileExplorerCommand(runtime.GOOS, path)
	if cmd == nil {
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Run()
}

func fileExplorerCommand(goos, path string) *exec.Cmd {
	switch goos {
	case "darwin":
		return exec.Command("open", "-R", path)
	case "windows":
		return exec.Command("explorer", "/select,"+path)
	default:
		return exec.Command("xdg-open", filepath.Dir(path))
	}
}

func writeCSV(path string, columns []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeJSON(path string, columns []string, rows [][]string) error {
	objects := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		obj := map[string]string{}
		for i, col := range columns {
			if i < len(row) {
				obj[col] = row[i]
			} else {
				obj[col] = ""
			}
		}
		objects = append(objects, obj)
	}
	data, err := json.MarshalIndent(objects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeText(path string, columns []string, rows [][]string) error {
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, val := range row {
			if i < len(widths) && len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}
	var b strings.Builder
	writeTextRow(&b, columns, widths)
	for i, width := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("-", width))
	}
	b.WriteByte('\n')
	for _, row := range rows {
		writeTextRow(&b, row, widths)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeTextRow(b *strings.Builder, row []string, widths []int) {
	for i, width := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		val := ""
		if i < len(row) {
			val = row[i]
		}
		b.WriteString(val)
		if pad := width - len(val); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	b.WriteByte('\n')
}

func resolveExportDir(dir string) string {
	if strings.TrimSpace(dir) != "" {
		return expandHome(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "crosspile")
	}
	return filepath.Join(home, "Downloads", "crosspile")
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
