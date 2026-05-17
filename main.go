package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wingitman/crosspile/internal/app"
	"github.com/wingitman/crosspile/internal/config"
	"github.com/wingitman/crosspile/internal/scanner"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "crosspile: config warning: %v\n", err)
	}

	m := app.New(cfg, scanner.Scan)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "crosspile: %v\n", err)
		os.Exit(1)
	}
}
