package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ntxinh/RepoLens/internal/config"
	"github.com/ntxinh/RepoLens/internal/launcher"
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

	// Global flags may precede the subcommand.
	args := os.Args[1:]
	var noColor bool
	for len(args) > 0 && args[0] == "--no-color" {
		noColor = true
		args = args[1:]
	}
	if noColor {
		lipgloss.SetColorProfile(0)
	}
	if len(args) == 0 {
		runTUI()
		return
	}

	var err error
	code := 1
	switch args[0] {
	case "scan":
		err = cmdScan(args[1:])
	case "add":
		err = cmdAdd(args[1:])
	case "remove":
		err = cmdRemove(args[1:])
	case "list":
		err = cmdList(args[1:])
	case "status":
		err = cmdStatus(args[1:])
	case "open":
		err = cmdOpen(args[1:])
	case "pick":
		err = cmdPick(args[1:])
	case "config":
		err = cmdConfig(args[1:])
	case "version":
		fmt.Println("repolens", version)
	case "--help", "-h", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "repolens: unknown command %q\n\n", args[0])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "repolens:", err)
		os.Exit(code)
	}
}

func ctx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func runTUI() {
	svc, err := loadService("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "repolens:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(tui.New(version, svc))
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

// cmdAdd registers a single repository path.
func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: repolens add PATH")
	}
	p, err := config.ExpandPath(fs.Arg(0))
	if err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	c, cancel := ctx()
	defer cancel()
	if _, _, err := svc.Runner.Detect(c, p); err != nil {
		return err
	}
	svc.Reg.Add(p)
	if err := svc.Reg.Save(); err != nil {
		return err
	}
	fmt.Println("Registered", p)
	return nil
}

// cmdRemove unregisters a repository without touching files.
func cmdRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: repolens remove PATH")
	}
	p, err := config.ExpandPath(fs.Arg(0))
	if err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	if !svc.Reg.Remove(p) {
		return fmt.Errorf("%s is not registered", p)
	}
	if err := svc.Reg.Save(); err != nil {
		return err
	}
	fmt.Println("Removed", p)
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
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\n",
			s.Path, branchLabel(s), abLabel(s), s.Conditions())
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

func abLabel(s repository.Status) string {
	if s.Err != nil || !s.HasCounts {
		return "—"
	}
	return fmt.Sprintf("+%d/-%d", s.Ahead, s.Behind)
}

// cmdOpen launches a configured tool for a repository.
// With no PATH it uses the current directory.
func cmdOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	tool := fs.String("tool", "editor", "editor|terminal|file_manager|git_tui|worktree_manager")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	abs, err := config.ExpandPath(path)
	if err != nil {
		return err
	}
	cmd, err := commandFor(svc, *tool)
	if err != nil {
		return err
	}
	c, cancel := ctx()
	defer cancel()
	if _, _, err := svc.Runner.Detect(c, abs); err != nil {
		return err
	}
	switch *tool {
	case "git_tui", "worktree_manager":
		// Interactive TUI tools inherit the terminal.
		ec, err := launcher.Command(cmd, abs)
		if err != nil {
			return err
		}
		ec.Stdin, ec.Stdout, ec.Stderr = os.Stdin, os.Stdout, os.Stderr
		return ec.Run()
	default:
		return launcher.Launch(cmd, abs)
	}
}

func commandFor(svc *service.Service, tool string) ([]string, error) {
	c := svc.Cfg.Commands
	var cmd []string
	switch tool {
	case "editor":
		cmd = c.Editor
	case "terminal":
		cmd = c.Terminal
	case "file_manager":
		cmd = c.FileManager
	case "git_tui":
		cmd = c.GitTUI
	case "worktree_manager":
		cmd = c.WorktreeManager
	default:
		return nil, fmt.Errorf("unknown tool %q (valid: editor, terminal, file_manager, git_tui, worktree_manager)", tool)
	}
	if len(cmd) == 0 {
		return nil, fmt.Errorf("commands.%s not configured", tool)
	}
	return cmd, nil
}

// cmdPick prints the absolute path of a registered repository matching
// QUERY (basename or substring). Exists for shell integration: fish's
// croot does `cd (repolens pick QUERY)`.
func cmdPick(args []string) error {
	fs := flag.NewFlagSet("pick", flag.ContinueOnError)
	configPath := fs.String("config", "", "alternate config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc, err := loadService(*configPath)
	if err != nil {
		return err
	}
	entries := svc.Reg.Entries()
	query := ""
	if fs.NArg() > 0 {
		query = fs.Arg(0)
	}
	matches := pickMatches(entries, query)
	switch len(matches) {
	case 0:
		return fmt.Errorf("no registered repository matches %q", query)
	case 1:
		fmt.Println(matches[0].Path)
	default:
		for _, e := range matches {
			fmt.Fprintln(os.Stderr, e.Path)
		}
		return fmt.Errorf("%d matches — be more specific", len(matches))
	}
	return nil
}

func pickMatches(entries []registry.Entry, query string) []registry.Entry {
	if query == "" {
		return entries
	}
	var out []registry.Entry
	for _, e := range entries {
		if filepath.Base(e.Path) == query {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		for _, e := range entries {
			if containsFold(e.Path, query) {
				out = append(out, e)
			}
		}
	}
	return out
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
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
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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

// Exit codes: 0 success, 1 runtime error, 2 usage error.
func usage() {
	fmt.Print(`repolens — local-first multi-repository Git status dashboard

Usage:
  repolens                          Launch the TUI dashboard
  repolens scan [ROOT...]           Discover and register repositories
  repolens add PATH                 Register one repository
  repolens remove PATH              Unregister (does not delete files)
  repolens list [--json]            List registered repositories
  repolens status [PATH...] [--json]  Show status (all if no PATH)
  repolens open [PATH] [--tool T]   Open repo in a configured tool
  repolens pick [QUERY]             Print matching repo path (for croot)
  repolens config init|path         Manage configuration
  repolens version                  Print version
  repolens [--no-color]             Disable ANSI colors

Exit codes: 0 ok · 1 error · 2 usage
`)
}
