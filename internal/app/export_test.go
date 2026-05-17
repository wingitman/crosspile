package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
