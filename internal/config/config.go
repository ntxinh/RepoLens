// Package config is the only package that reads TOML configuration.
// It resolves XDG paths, expands ~ and environment variables, and
// validates values with key-level error messages.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config mirrors repolens config.toml. Unknown keys are rejected.
type Config struct {
	General   General   `toml:"general"`
	Discovery Discovery `toml:"discovery"`
	Display   Display   `toml:"display"`
	Commands  Commands  `toml:"commands"`
}

type General struct {
	RefreshIntervalSeconds int    `toml:"refresh_interval_seconds"`
	DiscoveryMaxDepth      int    `toml:"discovery_max_depth"`
	FollowSymlinks         bool   `toml:"follow_symlinks"`
	ShowHiddenDirectories  bool   `toml:"show_hidden_directories"`
	DefaultSort            string `toml:"default_sort"`
	Theme                  string `toml:"theme"`
}

type Discovery struct {
	Roots   []string `toml:"roots"`
	Exclude []string `toml:"exclude"`
}

type Display struct {
	ShowPath        bool `toml:"show_path"`
	ShowUpstream    bool `toml:"show_upstream"`
	ShowAheadBehind bool `toml:"show_ahead_behind"`
	ShowLastCommit  bool `toml:"show_last_commit"`
}

type Commands struct {
	Editor          []string `toml:"editor"`
	Terminal        []string `toml:"terminal"`
	FileManager     []string `toml:"file_manager"`
	GitTUI          []string `toml:"git_tui"`
	WorktreeManager []string `toml:"worktree_manager"`
}

var validSorts = map[string]bool{
	"attention": true, "dirty": true, "name": true, "branch": true,
	"ahead_behind": true, "last_commit": true, "path": true,
}

// Default returns a usable configuration when no file exists yet.
func Default() Config {
	var c Config
	c.General.RefreshIntervalSeconds = 60
	c.General.DiscoveryMaxDepth = 4
	c.General.ShowHiddenDirectories = true
	c.General.DefaultSort = "attention"
	c.General.Theme = "auto"
	c.Discovery.Exclude = []string{
		"**/node_modules/**", "**/vendor/**", "**/target/**",
		"**/dist/**", "**/.cache/**",
	}
	c.Display.ShowPath = true
	c.Display.ShowUpstream = true
	c.Display.ShowAheadBehind = true
	c.Display.ShowLastCommit = true
	c.Commands.Editor = []string{"code", "{path}"}
	c.Commands.Terminal = []string{"ghostty", "--working-directory", "{path}"}
	c.Commands.FileManager = []string{"xdg-open", "{path}"}
	return c
}

// Paths returns the resolved XDG locations RepoLens uses.
func Paths() (configDir, dataDir, cacheDir, stateDir string) {
	xdg := func(env, fallback string) string {
		if v := os.Getenv(env); v != "" {
			return filepath.Join(v, "repolens")
		}
		return filepath.Join(home(), fallback, "repolens")
	}
	return xdg("XDG_CONFIG_HOME", ".config"),
		xdg("XDG_DATA_HOME", ".local/share"),
		xdg("XDG_CACHE_HOME", ".cache"),
		xdg("XDG_STATE_HOME", ".local/state")
}

// File returns the default config file path.
func File() string {
	dir, _, _, _ := Paths()
	return filepath.Join(dir, "config.toml")
}

// RegistryFile returns the registry path inside the data dir.
func RegistryFile() string {
	_, data, _, _ := Paths()
	return filepath.Join(data, "registry.json")
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

// Load reads and validates path. Missing file returns Default().
func Load(path string) (Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}
	d := toml.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return c, fmt.Errorf("config %s:%d:%d: %s", path, row, col, de.Error())
		}
		return c, fmt.Errorf("config %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return c, fmt.Errorf("config %s: %w", path, err)
	}
	return c, nil
}

// Validate checks enum fields and command shapes, naming the bad key.
func (c Config) Validate() error {
	if c.General.RefreshIntervalSeconds < 0 {
		return fmt.Errorf("general.refresh_interval_seconds: must be >= 0, got %d", c.General.RefreshIntervalSeconds)
	}
	if c.General.DiscoveryMaxDepth < 0 {
		return fmt.Errorf("general.discovery_max_depth: must be >= 0, got %d", c.General.DiscoveryMaxDepth)
	}
	if !validSorts[c.General.DefaultSort] {
		return fmt.Errorf("general.default_sort: unknown %q (valid: attention, dirty, name, branch, ahead_behind, last_commit, path)", c.General.DefaultSort)
	}
	switch c.General.Theme {
	case "auto", "dark", "light":
	default:
		return fmt.Errorf("general.theme: unknown %q (valid: auto, dark, light)", c.General.Theme)
	}
	for key, cmd := range map[string][]string{
		"commands.editor":           c.Commands.Editor,
		"commands.terminal":         c.Commands.Terminal,
		"commands.file_manager":     c.Commands.FileManager,
		"commands.git_tui":          c.Commands.GitTUI,
		"commands.worktree_manager": c.Commands.WorktreeManager,
	} {
		for i, arg := range cmd {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("%s: argument %d is empty", key, i)
			}
		}
	}
	return nil
}

// ExpandPath expands a leading ~ and $VARS, then absolutizes.
func ExpandPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	if strings.HasPrefix(p, "~/") || p == "~" {
		p = filepath.Join(home(), strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	p = os.Expand(p, func(name string) string {
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return "$" + name // left literal; validation reports it
	})
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("path %q: %w", p, err)
	}
	return abs, nil
}

// Roots returns discovery roots with all expansions applied.
func (c Config) Roots() ([]string, error) {
	out := make([]string, 0, len(c.Discovery.Roots))
	for _, r := range c.Discovery.Roots {
		p, err := ExpandPath(r)
		if err != nil {
			return nil, fmt.Errorf("discovery.roots: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}
