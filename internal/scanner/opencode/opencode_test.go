package opencode

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestScanDBReadsCurrentOpenCodeSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`create table session (id text primary key, project_id text not null, parent_id text, slug text not null, directory text not null, title text not null, version text not null, share_url text, summary_additions integer, summary_deletions integer, summary_files integer, summary_diffs text, revert text, permission text, time_created integer not null, time_updated integer not null, time_compacting integer, time_archived integer, workspace_id text)`,
		`create table message (id text primary key, session_id text not null, time_created integer not null, time_updated integer not null, data text not null)`,
		`create table part (id text primary key, message_id text not null, session_id text not null, time_created integer not null, time_updated integer not null, data text not null)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	if _, err := db.Exec(`insert into session (id, project_id, slug, directory, title, version, time_created, time_updated) values ('ses_1', 'proj_1', 'test-session', ?, 'Test Session', '1.0.0', 1772484277000, 1772484286000)`, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into message (id, session_id, time_created, time_updated, data) values ('msg_1', 'ses_1', 1772484277205, 1772484282116, '{"role":"assistant","time":{"created":1772484277205,"completed":1772484282116},"modelID":"claude-sonnet-4-6","providerID":"opencode","mode":"explore","agent":"explore","cost":0.0156477,"tokens":{"input":1,"output":462,"reasoning":0,"cache":{"read":5849,"write":1856}}}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`insert into part (id, message_id, session_id, time_created, time_updated, data) values ('part_1', 'msg_1', 'ses_1', 1772484277300, 1772484277300, '{"type":"text","text":"hello from opencode"}')`); err != nil {
		t.Fatal(err)
	}

	sessions, err := scanDB(context.Background(), dbPath, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.ID != "ses_1" || s.Title != "Test Session" || s.Mode != "explore" || s.Model != "claude-sonnet-4-6" || s.Provider != "opencode" {
		t.Fatalf("unexpected session metadata: %#v", s)
	}
	if s.Cost != 0.0156477 || s.TokensIn != 1 || s.TokensOut != 462 || s.TokensCacheRead != 5849 || s.TokensCacheWrite != 1856 {
		t.Fatalf("unexpected session usage: %#v", s)
	}
	if len(s.Messages) != 1 || len(s.Messages[0].Parts) != 1 || s.Messages[0].Parts[0].Text != "hello from opencode" {
		t.Fatalf("unexpected messages: %#v", s.Messages)
	}
}

func TestSessionHealthFlagsDefaultTitle(t *testing.T) {
	health, issues := sessionHealth("New session - 2026-08-19T20:18:09.653Z")
	if health != "degraded" || len(issues) != 1 {
		t.Fatalf("unexpected health: %q %#v", health, issues)
	}
}
