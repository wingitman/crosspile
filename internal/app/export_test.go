package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wingitman/crosspile/internal/analytics"
)

func TestWriteExportCSV(t *testing.T) {
	dir := t.TempDir()
	path, err := writeExport(dir, "ses_one", "message", "csv", []string{"id", "text"}, [][]string{{"1", "hello, world"}})
	if err != nil {
		t.Fatalf("writeExport csv: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if filepath.Ext(path) != ".csv" || !strings.Contains(string(data), "id,text") || !strings.Contains(string(data), `"hello, world"`) {
		t.Fatalf("unexpected csv %q at %s", string(data), path)
	}
}

func TestWriteExportJSON(t *testing.T) {
	dir := t.TempDir()
	path, err := writeExport(dir, "ses_one", "message", "json", []string{"id", "text"}, [][]string{{"1", "hello"}})
	if err != nil {
		t.Fatalf("writeExport json: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var rows []map[string]string
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, string(data))
	}
	if filepath.Ext(path) != ".json" || len(rows) != 1 || rows[0]["text"] != "hello" {
		t.Fatalf("unexpected json rows %#v at %s", rows, path)
	}
}

func TestWriteExportText(t *testing.T) {
	dir := t.TempDir()
	path, err := writeExport(dir, "ses_one", "message", "txt", []string{"id", "text"}, [][]string{{"1", "hello"}})
	if err != nil {
		t.Fatalf("writeExport text: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read text: %v", err)
	}
	if filepath.Ext(path) != ".txt" || !strings.Contains(string(data), "id") || !strings.Contains(string(data), "hello") {
		t.Fatalf("unexpected text %q at %s", string(data), path)
	}
}

func TestAnalyticsExportDataUsesCurrentMetrics(t *testing.T) {
	columns, rows := analyticsExportData(
		analytics.DimensionAgent,
		[]analytics.Metric{analytics.MetricSessions, analytics.MetricCost},
		[]analytics.Row{{Key: "opencode", Sessions: 2, Cost: 0.35}},
		analytics.Row{Sessions: 2, Cost: 0.35},
	)

	if strings.Join(columns, ",") != "agent,sessions,cost" {
		t.Fatalf("columns = %#v", columns)
	}
	if len(rows) != 2 || rows[0][0] != "opencode" || rows[0][1] != "2" || rows[0][2] != "$0.3500" {
		t.Fatalf("unexpected rows %#v", rows)
	}
	if rows[1][0] != "total" || rows[1][1] != "2" {
		t.Fatalf("missing total row %#v", rows)
	}
}

func TestWriteAnalyticsExportText(t *testing.T) {
	dir := t.TempDir()
	path, err := writeAnalyticsExport(dir, analytics.DimensionAgent, analytics.BucketAll, "txt", []string{"agent", "sessions"}, [][]string{{"opencode", "2"}, {"total", "2"}})
	if err != nil {
		t.Fatalf("write analytics text: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read text: %v", err)
	}
	if filepath.Ext(path) != ".txt" || !strings.Contains(string(data), "agent") || !strings.Contains(string(data), "opencode") || !strings.Contains(string(data), "total") {
		t.Fatalf("unexpected text export %q at %s", string(data), path)
	}
}

func TestFileExplorerCommand(t *testing.T) {
	tests := []struct {
		goos string
		path string
		want []string
	}{
		{"darwin", "/tmp/crosspile/out.csv", []string{"open", "-R", "/tmp/crosspile/out.csv"}},
		{"windows", `C:\tmp\crosspile\out.csv`, []string{"explorer", `/select,C:\tmp\crosspile\out.csv`}},
		{"linux", "/tmp/crosspile/out.csv", []string{"xdg-open", "/tmp/crosspile"}},
	}
	for _, tt := range tests {
		cmd := fileExplorerCommand(tt.goos, tt.path)
		if cmd == nil || strings.Join(cmd.Args, "|") != strings.Join(tt.want, "|") {
			t.Fatalf("%s command = %#v, want %#v", tt.goos, cmd.Args, tt.want)
		}
	}
}
