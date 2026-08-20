package codewhale

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRetainsCorruptedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".codewhale", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions, _ := Scan(t.Context(), nil)
	if len(sessions) != 1 || sessions[0].Health != "corrupted" {
		t.Fatalf("expected corrupted session, got %#v", sessions)
	}
}

func TestScanReadsMetadataAndContentBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".codewhale", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session.json")
	data := `{"metadata":{"id":"cw1","title":"CodeWhale test","workspace":"/work/project","model":"deepseek-v4-pro","mode":"agent"},"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions, warnings := Scan(t.Context(), []string{"/work"})
	if len(warnings) != 0 || len(sessions) != 1 {
		t.Fatalf("unexpected scan: %#v %#v", sessions, warnings)
	}
	s := sessions[0]
	if s.ID != "cw1" || s.Title != "CodeWhale test" || s.Directory != "/work/project" || s.PromptText() != "hello" {
		t.Fatalf("unexpected session: %#v", s)
	}
}
