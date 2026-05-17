package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/crosspile/internal/app"
	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/scanner"
	"github.com/wingitman/crosspile/internal/updatecheck"
)

var (
	version = "dev"
	origin  = ""
	repoDir = ""
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--default" {
		if err := config.ResetDefault(); err != nil {
			fmt.Fprintf(os.Stderr, "crosspile: failed to reset config: %v\n", err)
			os.Exit(1)
		}
		path, _ := config.ConfigPath()
		fmt.Printf("crosspile: config reset to factory defaults\n  %s\n", path)
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "--set-repo-dir" {
		metaOrigin := origin
		if len(os.Args) > 3 && os.Args[3] != "" {
			metaOrigin = os.Args[3]
		}
		metaCommit := version
		if len(os.Args) > 4 && os.Args[4] != "" {
			metaCommit = os.Args[4]
		}
		installSource := "unknown"
		if len(os.Args) > 5 && os.Args[5] != "" {
			installSource = os.Args[5]
		}
		platform := updatecheck.Platform()
		if len(os.Args) > 6 && os.Args[6] != "" {
			platform = os.Args[6]
		}
		if err := config.WriteInstallMeta(os.Args[2], metaOrigin, metaCommit, installSource, platform); err != nil {
			fmt.Fprintf(os.Stderr, "crosspile: failed to write install metadata: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "crosspile: config warning: %v\n", err)
	}

	m := app.New(cfg, scanner.Scan, origin, version, repoDir)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "crosspile: %v\n", err)
		os.Exit(1)
	}
}
