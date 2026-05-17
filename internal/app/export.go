package app

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
		return statusMsg("exported " + path)
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
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
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
