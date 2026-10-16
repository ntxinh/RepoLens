package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ntxinh/RepoLens/internal/config"
	"github.com/ntxinh/RepoLens/internal/registry"
	"github.com/ntxinh/RepoLens/internal/repository"
	"github.com/ntxinh/RepoLens/internal/service"
	"github.com/ntxinh/RepoLens/internal/tui"
)

var version = "dev"

const (
	gitTimeout       = 10 * time.Second
	maxConcurrentGit = 8
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}
	var err error
	switch os.Args[1] {
	case "scan":
		err = cmdScan(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "config":
		err = cmdConfig(os.Args[2:])
	case "version":
		fmt.Println("repolens", version)
	case "--help", "-h", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "repolens: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "repolens:", err)
		os.Exit(1)
	}
}

func ctx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func runTUI() {
	p := tea.NewProgram(tui.New(version))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "repolens:", err)
		os.Exit(1)
	}
}

func loadService(configPath string) (*service.Service, error) {
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.File()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	reg, err := registry.Load(config.RegistryFile())
	if err != nil {
		return nil, err
	}
	return service.New(cfg, reg, gitTimeout, maxConcurrentGit), nil
}

func cmdScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	c, cancel := ctx()
	defer cancel()
	n, err := svc.ScanRoots(c, fs.Args())
	if err != nil {
		return fmt.Errorf("scanned %d repos, then: %w", n, err)
	}
	fmt.Printf("Registered %d repositories (%s)\n", n, svc.Reg.Path())
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(svc.Reg.Entries())
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, e := range svc.Reg.Entries() {
		mark := ""
		if e.Missing {
			mark = " [missing]"
		}
		fmt.Fprintf(w, "%s%s\n", e.Path, mark)
	}
	return w.Flush()
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	c, cancel := ctx()
	defer cancel()

	var statuses []repository.Status
	if paths := fs.Args(); len(paths) > 0 {
		for _, p := range paths {
			abs, err := config.ExpandPath(p)
			if err != nil {
				return err
			}
			statuses = append(statuses, svc.Refresh(c, abs))
		}
	} else {
		statuses = svc.RefreshAll(c)
	}

	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(statuses)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, s := range statuses {
		fmt.Fprintf(w, "%s\t%s\t+%d/-%d\t%v\n",
			s.Path, branchLabel(s), s.Ahead, s.Behind, s.Conditions())
	}
	return w.Flush()
}

func branchLabel(s repository.Status) string {
	if s.Err != nil {
		return "!"
	}
	if s.Detached {
		return "(detached)"
	}
	return s.Branch
}

func cmdConfig(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: repolens config <init|path>")
	}
	switch args[0] {
	case "path":
		fmt.Println(config.File())
	case "init":
		path := config.File()
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists at %s", path)
		}
		if err := os.MkdirAll(dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(defaultConfigDoc), 0o644); err != nil {
			return err
		}
		fmt.Println("Wrote", path)
	default:
		return fmt.Errorf("usage: repolens config <init|path>")
	}
	return nil
}

func dir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

const defaultConfigDoc = `# RepoLens configuration — see docs/CONFIGURATION.md

[general]
refresh_interval_seconds = 60
discovery_max_depth = 4
follow_symlinks = false
show_hidden_directories = true
default_sort = "attention"
theme = "auto"

[discovery]
roots = []
exclude = ["**/node_modules/**", "**/vendor/**", "**/target/**", "**/dist/**", "**/.cache/**"]

[display]
show_path = true
show_upstream = true
show_ahead_behind = true
show_last_commit = true

[commands]
editor = ["code", "{path}"]
terminal = ["ghostty", "--working-directory", "{path}"]
file_manager = ["xdg-open", "{path}"]
# git_tui = ["lazygit"]
# worktree_manager = ["wtm", "{path}"]
`

func usage() {
	fmt.Print(`repolens — local-first multi-repository Git status dashboard

Usage:
  repolens                      Launch the TUI dashboard
  repolens scan [ROOT...]       Discover and register repositories
  repolens list [--json]        List registered repositories
  repolens status [PATH...]     Show repository status (all if none given)
  repolens config init|path     Manage configuration
  repolens version              Print version

Flags (per subcommand): --config PATH, --json, --no-color
`)
}
