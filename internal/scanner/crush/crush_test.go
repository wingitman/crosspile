package crush

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestScanDiscoversConfiguredXDGPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	path := filepath.Join(root, "crush", "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"session_id":"c1","cwd":"`+root+`","prompt":"hello"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions, warnings := Scan(t.Context(), nil)
	if len(warnings) != 0 || len(sessions) != 1 || sessions[0].Agent != "crush" {
		t.Fatalf("unexpected scan: %#v %#v", sessions, warnings)
	}
}

func TestScanDiscoversNativeSQLiteFromProjectsRegistry(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	dbPath := filepath.Join(workspace, ".crush", "crush.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCrushDB(t, dbPath, `
CREATE TABLE sessions (id TEXT, title TEXT, prompt_tokens INTEGER, completion_tokens INTEGER, cost REAL, created_at INTEGER, updated_at INTEGER);
CREATE TABLE messages (id TEXT, session_id TEXT, role TEXT, parts TEXT, model TEXT, provider TEXT, created_at INTEGER, updated_at INTEGER);
INSERT INTO sessions VALUES ('s1', 'Native', 12, 8, 0.25, 1700000000, 1700000010);
INSERT INTO messages VALUES ('m1', 's1', 'user', '[{"type":"text","text":"hello"}]', 'model-x', 'provider-y', 1700000000, 1700000000);
INSERT INTO messages VALUES ('m2', 's1', 'assistant', '[{"type":"tool","name":"read_file"},{"type":"file","path":"main.go"}]', 'model-x', 'provider-y', 1700000001, 1700000001);`)
	registry, _ := json.Marshal([]map[string]string{{"path": workspace}})
	if err := os.WriteFile(filepath.Join(root, "projects.json"), registry, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRUSH_DATA_DIR", root)
	sessions, warnings := Scan(t.Context(), nil)
	if len(warnings) != 0 || len(sessions) != 1 {
		t.Fatalf("unexpected scan: %#v %#v", sessions, warnings)
	}
	s := sessions[0]
	if s.ID != "s1" || s.Directory != workspace || s.TokensIn != 12 || s.Model != "model-x" || len(s.Messages) != 2 {
		t.Fatalf("unexpected session: %#v", s)
	}
	if len(s.Tools) != 1 || len(s.Files) != 1 {
		t.Fatalf("parts not indexed: %#v %#v", s.Tools, s.Files)
	}
}

func TestSQLiteSchemaDriftAndInvalidPartsAreDegraded(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "nested", "crush.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCrushDB(t, dbPath, `
CREATE TABLE sessions (id TEXT, created_at INTEGER);
CREATE TABLE messages (session_id TEXT, role TEXT, parts TEXT);
INSERT INTO sessions VALUES ('drift', 1700000000);
INSERT INTO messages VALUES ('drift', 'user', '{not-json}');`)
	t.Setenv("CRUSH_DATA_DIR", root)
	sessions, warnings := Scan(t.Context(), nil)
	if len(warnings) != 0 || len(sessions) != 1 {
		t.Fatalf("unexpected scan: %#v %#v", sessions, warnings)
	}
	if sessions[0].Health != "degraded" || len(sessions[0].Issues) == 0 || len(sessions[0].Messages) != 1 {
		t.Fatalf("drift/invalid parts not preserved: %#v", sessions[0])
	}
}

func writeCrushDB(t *testing.T, path, script string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(script); err != nil {
		t.Fatal(err)
	}
}
