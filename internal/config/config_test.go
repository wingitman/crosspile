package config

import (
	"strings"
	"testing"
)

func TestBuildTOMLUsesCrossfileShape(t *testing.T) {
	cfg := Default()
	cfg.Locations = []Location{{Name: "Work", Path: "/home/example/Work"}}
	toml := BuildTOML(cfg)
	for _, want := range []string{"[[locations]]", "[agents]", "[display]", "[keybinds]", "opencode = true"} {
		if !strings.Contains(toml, want) {
			t.Fatalf("expected %q in TOML:\n%s", want, toml)
		}
	}
}
