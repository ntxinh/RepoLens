package git

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// parsePorcelainV2 fills snap's branch/upstream/counts/Files from
// `git status --porcelain=v2 --branch -z` output.
//
// Record forms (NUL-separated):
//
//	# branch.head <name|(detached)>
//	# branch.oid <oid|(initial)>
//	# branch.upstream <upstream>
//	# branch.ab +<ahead> -<behind>
//	1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
//	2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <Xscore> <path><sep><origPath>
//	u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
//	? <path>
//	! <path>
func parsePorcelainV2(out []byte, snap *Snapshot) {
	fields := bytes.Split(out, []byte{0})
	for i := 0; i < len(fields); i++ {
		line := fields[i]
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '#':
			parseBranchHeader(line, snap)
		case '1', 'u':
			e := parseOrdinaryRecord(line)
			snap.Files = append(snap.Files, e)
		case '2':
			e, orig := parseRenameRecord(line)
			// The original path is the *next* NUL-separated field.
			if i+1 < len(fields) {
				e.OrigPath = sanitizeField(fields[i+1])
				i++
			}
			_ = orig
			snap.Files = append(snap.Files, e)
		case '?':
			snap.Files = append(snap.Files, FileEntry{Path: sanitizeField(line[2:]), X: '.', Y: '.', Kind: '?'})
		case '!':
			// ignored entries are not emitted without --ignored; skip defensively
		}
	}
}

func parseBranchHeader(line []byte, snap *Snapshot) {
	s := sanitizeField(line)
	switch {
	case strings.HasPrefix(s, "# branch.head "):
		head := s[len("# branch.head "):]
		if head == "(detached)" {
			snap.Detached = true
		} else {
			snap.Branch = head
		}
	case strings.HasPrefix(s, "# branch.upstream "):
		snap.Upstream = s[len("# branch.upstream "):]
		snap.HasUpstream = true
	case strings.HasPrefix(s, "# branch.ab "):
		var ahead, behind int
		if n, err := fmtSscanf(s[len("# branch.ab "):], &ahead, &behind); err == nil && n == 2 {
			snap.Ahead, snap.Behind, snap.HasCounts = ahead, behind, true
		}
	}
}

// fmtSscanf parses "+<ahead> -<behind>" without importing fmt's full scan table.
func fmtSscanf(s string, a, b *int) (int, error) {
	var sign int
	var err error
	if !strings.HasPrefix(s, "+") {
		return 0, strconv.ErrSyntax
	}
	rest := s[1:]
	idx := strings.Index(rest, " -")
	if idx < 0 {
		return 0, strconv.ErrSyntax
	}
	*a, err = strconv.Atoi(rest[:idx])
	if err != nil {
		return 0, err
	}
	*b, err = strconv.Atoi(rest[idx+2:])
	if err != nil {
		return 0, err
	}
	_ = sign
	return 2, nil
}

// parseOrdinaryRecord handles '1' and 'u' records:
// "1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>"
// "u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>"
func parseOrdinaryRecord(line []byte) FileEntry {
	e := FileEntry{Kind: line[0]}
	// Fields are space-separated up to the path, which is last and may
	// contain spaces.
	parts := bytes.SplitN(line, []byte{' '}, -1)
	// For '1' there are 9 fields total; for 'u' 11. XY is field 1 either way.
	if len(parts) >= 2 && len(parts[1]) == 2 {
		e.X, e.Y = parts[1][0], parts[1][1]
	}
	if len(parts) >= 2 {
		e.Path = sanitizeField(parts[len(parts)-1])
	}
	return e
}

// parseRenameRecord handles '2' records:
// "2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <Xscore> <path>"
// (the orig path follows as the next NUL field; caller consumes it)
func parseRenameRecord(line []byte) (e FileEntry, _ string) {
	e.Kind = '2'
	parts := bytes.SplitN(line, []byte{' '}, -1)
	if len(parts) >= 2 && len(parts[1]) == 2 {
		e.X, e.Y = parts[1][0], parts[1][1]
	}
	if len(parts) >= 1 {
		e.Path = sanitizeField(parts[len(parts)-1])
	}
	return e, ""
}

// parseLogLine parses one `--format=%h%x00%s%x00%an%x00%ct` record.
func parseLogLine(out []byte) *Commit {
	// Drop trailing newline/git separator.
	out = bytes.TrimRight(out, "\n")
	fields := bytes.Split(out, []byte{0})
	if len(fields) < 4 {
		return nil
	}
	ts, err := strconv.ParseInt(sanitizeField(fields[3]), 10, 64)
	if err != nil {
		return nil
	}
	return &Commit{
		Hash:      sanitizeField(fields[0]),
		Subject:   sanitizeField(fields[1]),
		Author:    sanitizeField(fields[2]),
		Timestamp: time.Unix(ts, 0),
	}
}

// sanitizeField strips characters that are unsafe for later terminal
// rendering or structured comparisons: control bytes except NUL boundaries
// are kept intact here (paths legitimately contain them); instead this
// decodes the git "quoted" form if present and validates UTF-8.
func sanitizeField(b []byte) string {
	s := string(b)
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") && len(s) >= 2 {
		if unq, err := strconv.Unquote(s); err == nil {
			s = unq
		}
	}
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	return s
}
