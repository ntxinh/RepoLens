// Package launcher spawns configured external tools — editor, terminal,
// file manager, Git TUI, worktree manager — via executable + argument
// array. It never invokes a shell and never interpolates a path into a
// command string.
package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// PathPlaceholder marks where the repository path goes in an arg list.
const PathPlaceholder = "{path}"

// ErrMissingExecutable reports a configured command whose binary is not
// on PATH.
type ErrMissingExecutable struct{ Name string }

func (e *ErrMissingExecutable) Error() string {
	return fmt.Sprintf("executable %q not found on PATH", e.Name)
}

// substitute replaces {path} in args; a command without the placeholder
// gets the path appended (e.g. ["lazygit"] → lazygit <path>).
func substitute(cmd []string, path string) ([]string, error) {
	if len(cmd) == 0 {
		return nil, errors.New("empty command")
	}
	out := make([]string, len(cmd))
	found := false
	for i, a := range cmd {
		if a == PathPlaceholder {
			out[i] = path
			found = true
		} else {
			out[i] = strings.ReplaceAll(a, PathPlaceholder, path)
			if a != out[i] {
				found = true
			}
		}
	}
	if !found {
		out = append(out, path)
	}
	return out, nil
}

// Launch runs cmd (executable + args, {path} substituted) detached from
// the TUI: own process group, stdio to /dev/null so it can't corrupt the
// display. Terminal emulators spawned this way open new windows under
// Wayland/Niri.
func Launch(cmd []string, path string) error {
	args, err := substitute(cmd, path)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return &ErrMissingExecutable{Name: args[0]}
	}
	c := exec.Command(args[0], args[1:]...)
	c.Dir = path
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()
	c.Stdin, c.Stdout, c.Stderr = devNull, devNull, devNull
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", args[0], err)
	}
	// Reap in background so we don't leave zombies.
	go c.Wait()
	return nil
}

// Command builds an *exec.Cmd for interactive TUI tools (Git TUI,
// worktree manager). The caller runs it via tea.ExecProcess so it
// inherits the terminal; the tool exits, the dashboard resumes.
func Command(cmd []string, path string) (*exec.Cmd, error) {
	args, err := substitute(cmd, path)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return nil, &ErrMissingExecutable{Name: args[0]}
	}
	c := exec.Command(args[0], args[1:]...)
	c.Dir = path
	return c, nil
}
