package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ntxinh/RepoLens/internal/tui"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println("repolens", version)
			return
		case "--help", "-h", "help":
			usage()
			return
		default:
			fmt.Fprintf(os.Stderr, "repolens: unknown command %q\n\n", os.Args[1])
			usage()
			os.Exit(2)
		}
	}

	p := tea.NewProgram(tui.New(version))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "repolens:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`repolens — local-first multi-repository Git status dashboard

Usage:
  repolens            Launch the TUI dashboard
  repolens version    Print version
  repolens --help     Show this help

Further subcommands (scan, list, status, config) are planned — see docs/repolens-spec.md.
`)
}
