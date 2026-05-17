package config

import (
	"strings"
	"testing"
)

func TestBuildTOMLUsesCrossfileShape(t *testing.T) {
	cfg := Default()
	cfg.Locations = []Location{{Name: "Work", Path: "/home/example/Work"}}
	toml := BuildTOML(cfg)
	for _, want := range []string{"[[locations]]", "[agents]", "[display]", "[keybinds]", "opencode = true", `export       = "x"`} {
		if !strings.Contains(toml, want) {
			t.Fatalf("expected %q in TOML:\n%s", want, toml)
		}
	}
	for _, notWant := range []string{"export_text", "export_json"} {
		if strings.Contains(toml, notWant) {
			t.Fatalf("did not expect %q in TOML:\n%s", notWant, toml)
		}
	}
}

func TestApplyDefaultsUsesLegacyExportCSV(t *testing.T) {
	cfg := Default()
	cfg.Keybinds.Export = ""
	cfg.Keybinds.ExportCSV = "c"
	applyDefaults(cfg)
	if cfg.Keybinds.Export != "c" {
		t.Fatalf("export = %q, want legacy export_csv", cfg.Keybinds.Export)
	}
}
