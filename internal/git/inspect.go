package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// RecentCommits returns up to n latest local commits, newest first.
// Repositories with no commits return an empty slice, not an error.
func (r *Runner) RecentCommits(ctx context.Context, path string, n int) ([]Commit, error) {
	out, err := r.Run(ctx, "-C", path, "log", fmt.Sprintf("-%d", n), "--format=%h%x00%s%x00%an%x00%ct%x00")
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) && isNoCommitsError(ce) {
			return nil, nil
		}
		return nil, fmt.Errorf("log %s: %w", path, err)
	}
	// Fields arrive as groups of 4 NUL-separated values.
	fields := bytes.Split(out, []byte{0})
	var f []string
	for _, b := range fields {
		if s := sanitizeField(bytes.TrimRight(b, "\n")); s != "" {
			f = append(f, s)
		}
	}
	var commits []Commit
	for i := 0; i+3 < len(f); i += 4 {
		ts, err := strconv.ParseInt(f[i+3], 10, 64)
		if err != nil {
			continue
		}
		commits = append(commits, Commit{
			Hash: f[i], Subject: f[i+1], Author: f[i+2], Timestamp: time.Unix(ts, 0),
		})
	}
	return commits, nil
}

// MaxDiffBytes caps diff output; longer diffs are marked truncated.
const MaxDiffBytes = 256 * 1024

// DiffResult is a read-only diff snapshot.
type DiffResult struct {
	Staged    []byte
	Unstaged  []byte
	Truncated bool
}

// Diff returns staged and unstaged diffs (read-only). Output is capped at
// MaxDiffBytes; Truncated reports when content was cut.
func (r *Runner) Diff(ctx context.Context, path string) (*DiffResult, error) {
	res := &DiffResult{}
	for _, spec := range []struct {
		args []string
		dst  *[]byte
	}{
		{[]string{"diff", "--staged"}, &res.Staged},
		{[]string{"diff"}, &res.Unstaged},
	} {
		out, err := r.Run(ctx, append([]string{"-C", path}, spec.args...)...)
		if err != nil {
			return nil, fmt.Errorf("diff %s: %w", path, err)
		}
		if len(out) > MaxDiffBytes {
			out = out[:MaxDiffBytes]
			res.Truncated = true
		}
		*spec.dst = out
	}
	return res, nil
}
