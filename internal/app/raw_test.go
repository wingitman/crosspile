package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wingitman/crosspile/internal/model"
	_ "modernc.org/sqlite"
)

func TestLoadSQLiteRawPagesLargeTables(t *testing.T) {
	path := makeRawTestDB(t, 205)
	s := model.Session{ID: "s1", Source: path, SourceKind: "sqlite"}

	data, err := loadRawData(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	message := rawTableByName(t, data, "message")
	if message.Total != 205 {
		t.Fatalf("message total = %d, want 205", message.Total)
	}
	if got := len(message.Rows); got != rawPageSize {
		t.Fatalf("first page row count = %d, want %d", got, rawPageSize)
	}
	if message.Page != 0 || message.rawPageCount() != 2 {
		t.Fatalf("page metadata = page %d count %d, want page 0 count 2", message.Page, message.rawPageCount())
	}

	page, err := loadRawTablePage(context.Background(), s, "message", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(page.Rows); got != 5 {
		t.Fatalf("second page row count = %d, want 5", got)
	}
	if page.Rows[0][0] != "m200" {
		t.Fatalf("second page first id = %q, want m200", page.Rows[0][0])
	}

	clamped, err := loadRawTablePage(context.Background(), s, "message", 99)
	if err != nil {
		t.Fatal(err)
	}
	if clamped.Page != 1 || len(clamped.Rows) != 5 {
		t.Fatalf("clamped page = %d rows %d, want page 1 rows 5", clamped.Page, len(clamped.Rows))
	}
}

func TestRawFullValueStringDoesNotFormatJSON(t *testing.T) {
	raw := `{"value":"x"}`
	got := rawFullValueString(raw)
	if got != raw {
		t.Fatal("raw JSON should not be pretty-printed")
	}
}

func TestRawCellPreviewLimitsVisibleContent(t *testing.T) {
	got := rawCellPreview(strings.Repeat("x", 100) + "\n" + strings.Repeat("y", 100))
	if len([]rune(got)) > rawPreviewChars {
		t.Fatalf("preview length = %d, want <= %d", len([]rune(got)), rawPreviewChars)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("preview contains newline: %q", got)
	}
}

func TestWriteRawCellTempWritesFullContent(t *testing.T) {
	value := strings.Repeat("cell-value\n", 100)
	path, err := writeRawCellTemp("message", "data", 12, value)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != value {
		t.Fatal("raw cell temp file did not contain full content")
	}
}

func makeRawTestDB(t *testing.T, messages int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "raw.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`create table session (id text primary key, title text)`,
		`create table message (id text, session_id text, time_created integer, data text)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`insert into session (id, title) values ('s1', 'test')`); err != nil {
		t.Fatal(err)
	}
	for i := range messages {
		id := fmt.Sprintf("m%03d", i)
		if _, err := db.Exec(`insert into message (id, session_id, time_created, data) values (?, 's1', ?, ?)`, id, i, fmt.Sprintf(`{"i":%d}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func rawTableByName(t *testing.T, data rawData, name string) rawTable {
	t.Helper()
	for _, table := range data.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("raw table %q not found", name)
	return rawTable{}
}
