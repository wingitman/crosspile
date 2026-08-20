package scanner

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/model"
	"github.com/wingitman/crosspile/internal/scanner/claude"
	"github.com/wingitman/crosspile/internal/scanner/codewhale"
	"github.com/wingitman/crosspile/internal/scanner/crush"
	"github.com/wingitman/crosspile/internal/scanner/generic"
	"github.com/wingitman/crosspile/internal/scanner/opencode"
)

type Result struct {
	Sessions []model.Session
	Warnings []string
	Scanned  time.Time
}

func Scan(ctx context.Context, cfg *config.Config) Result {
	var out Result
	out.Scanned = time.Now()
	locations := locationPaths(cfg)

	if cfg.Agents.OpenCode {
		sessions, warnings := opencode.ScanMetadata(ctx, locations)
		out.Sessions = append(out.Sessions, sessions...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	if cfg.Agents.Claude {
		sessions, warnings := claude.Scan(ctx, locations)
		out.Sessions = append(out.Sessions, sessions...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	if cfg.Agents.Generic {
		sessions, warnings := generic.Scan(ctx, locations)
		out.Sessions = append(out.Sessions, sessions...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	if cfg.Agents.CodeWhale {
		sessions, warnings := codewhale.Scan(ctx, locations)
		out.Sessions = append(out.Sessions, sessions...)
		out.Warnings = append(out.Warnings, warnings...)
	}
	if cfg.Agents.Crush {
		sessions, warnings := crush.Scan(ctx, locations)
		out.Sessions = append(out.Sessions, sessions...)
		out.Warnings = append(out.Warnings, warnings...)
	}

	out.Sessions = dedupe(out.Sessions)
	annotateLocations(out.Sessions, cfg.Locations)
	sort.SliceStable(out.Sessions, func(i, j int) bool {
		return out.Sessions[i].UpdatedAt.After(out.Sessions[j].UpdatedAt)
	})
	return out
}

// Hydrate loads expensive transcript data after metadata has reached the UI.
func Hydrate(ctx context.Context, sessions []model.Session) ([]model.Session, []string) {
	return opencode.Hydrate(ctx, sessions)
}

func annotateLocations(sessions []model.Session, locations []config.Location) {
	for i := range sessions {
		for _, loc := range locations {
			if loc.Path != "" && InLocations(sessions[i].Directory, []string{loc.Path}) {
				sessions[i].LocationName = loc.Name
				sessions[i].LocationPath = loc.Path
				break
			}
		}
	}
}

func locationPaths(cfg *config.Config) []string {
	var paths []string
	for _, loc := range cfg.Locations {
		if loc.Path == "" {
			continue
		}
		path, err := filepath.Abs(loc.Path)
		if err == nil {
			path = filepath.Clean(path)
		}
		paths = append(paths, path)
	}
	return paths
}

func InLocations(path string, locations []string) bool {
	if len(locations) == 0 {
		return true
	}
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = filepath.Clean(abs)
	}
	for _, loc := range locations {
		if loc == "" {
			continue
		}
		if path == loc {
			return true
		}
		rel, err := filepath.Rel(loc, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func Warning(err error) []string {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return []string{err.Error()}
}

func dedupe(sessions []model.Session) []model.Session {
	seen := map[string]bool{}
	out := sessions[:0]
	for _, s := range sessions {
		key := s.Agent + "\x00" + s.ID + "\x00" + s.Directory
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}
