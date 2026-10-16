package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.General.DefaultSort != "attention" || c.General.DiscoveryMaxDepth != 4 {
		t.Fatalf("expected defaults, got %+v", c.General)
	}
}

func TestLoadValid(t *testing.T) {
	p := writeCfg(t, `
[general]
default_sort = "name"
theme = "dark"
discovery_max_depth = 2
[discovery]
roots = ["~/Projects"]
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.General.DefaultSort != "name" || c.General.Theme != "dark" || c.General.DiscoveryMaxDepth != 2 {
		t.Fatalf("got %+v", c.General)
	}
	roots, err := c.Roots()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if roots[0] != filepath.Join(home, "Projects") {
		t.Fatalf("got %q", roots[0])
	}
}

func TestInvalidSort(t *testing.T) {
	p := writeCfg(t, "[general]\ndefault_sort = \"bogus\"\n")
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "general.default_sort") {
		t.Fatalf("expected key-level error, got %v", err)
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	p := writeCfg(t, "[general]\nbogus_key = 1\n")
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected unknown-field error")
	}
}

func TestEmptyCommandArg(t *testing.T) {
	p := writeCfg(t, "[commands]\neditor = [\"code\", \"\"]\n")
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "commands.editor") {
		t.Fatalf("expected key-level error, got %v", err)
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got, err := ExpandPath("~/x")
	if err != nil || got != filepath.Join(home, "x") {
		t.Fatalf("~ expansion: %q %v", got, err)
	}
	t.Setenv("REPOLENS_TEST_DIR", "/tmp/rt")
	got, err = ExpandPath("$REPOLENS_TEST_DIR/sub")
	if err != nil || got != "/tmp/rt/sub" {
		t.Fatalf("env expansion: %q %v", got, err)
	}
	got, err = ExpandPath("relative/path")
	if err != nil || !filepath.IsAbs(got) {
		t.Fatalf("abs: %q %v", got, err)
	}
}

func TestXDGPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xc")
	t.Setenv("XDG_DATA_HOME", "/tmp/xd")
	t.Setenv("XDG_CACHE_HOME", "/tmp/xch")
	t.Setenv("XDG_STATE_HOME", "/tmp/xs")
	c, d, ch, s := Paths()
	if c != "/tmp/xc/repolens" || d != "/tmp/xd/repolens" || ch != "/tmp/xch/repolens" || s != "/tmp/xs/repolens" {
		t.Fatalf("got %q %q %q %q", c, d, ch, s)
	}
}
