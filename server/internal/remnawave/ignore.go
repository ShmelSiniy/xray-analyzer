package remnawave

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// IgnoreList decides whether a Remnawave user should be excluded from all
// statistics and monitoring. Entries are loaded from a plain-text file (one
// pattern per line, "#" comments and blank lines ignored) and matched against
// the username, case-insensitively. Supported pattern forms:
//
//	tech_user        exact match
//	tech_user*       prefix match (everything starting with "tech_user")
//	*_bot            suffix match (everything ending with "_bot")
//	*service*        substring match
//	*                matches everything (use with care)
//
// The file is re-read on demand via Reload, so changes take effect on the next
// sync cycle without a rebuild or restart.
type IgnoreList struct {
	path string

	mu       sync.RWMutex
	exact    map[string]bool
	prefixes []string
	suffixes []string
	contains []string
	all      bool
}

// NewIgnoreList creates an ignore list backed by the given file path. An empty
// path (or a missing file) yields an empty list that ignores nobody. The file
// is loaded immediately; callers should also call Reload before each sync to
// pick up edits.
func NewIgnoreList(path string) *IgnoreList {
	l := &IgnoreList{path: path, exact: make(map[string]bool)}
	l.Reload()
	return l
}

// Reload re-reads the backing file. A missing or empty path leaves the list
// empty (ignores nobody). A read error is treated the same way — matching is
// fail-open so a bad file never hides users unexpectedly. Reload is safe for
// concurrent use with Match.
func (l *IgnoreList) Reload() {
	exact := make(map[string]bool)
	var prefixes, suffixes, contains []string
	all := false

	if l.path != "" {
		if f, err := os.Open(l.path); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				pat := strings.ToLower(line)
				switch {
				case pat == "*":
					all = true
				case strings.HasPrefix(pat, "*") && strings.HasSuffix(pat, "*") && len(pat) > 2:
					contains = append(contains, strings.Trim(pat, "*"))
				case strings.HasSuffix(pat, "*"):
					prefixes = append(prefixes, strings.TrimSuffix(pat, "*"))
				case strings.HasPrefix(pat, "*"):
					suffixes = append(suffixes, strings.TrimPrefix(pat, "*"))
				default:
					exact[pat] = true
				}
			}
			f.Close()
		}
	}

	l.mu.Lock()
	l.exact = exact
	l.prefixes = prefixes
	l.suffixes = suffixes
	l.contains = contains
	l.all = all
	l.mu.Unlock()
}

// Match reports whether the given username is on the ignore list. Matching is
// case-insensitive. An empty username is never ignored.
func (l *IgnoreList) Match(username string) bool {
	if username == "" {
		return false
	}
	u := strings.ToLower(username)

	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.all {
		return true
	}
	if l.exact[u] {
		return true
	}
	for _, p := range l.prefixes {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	for _, s := range l.suffixes {
		if strings.HasSuffix(u, s) {
			return true
		}
	}
	for _, c := range l.contains {
		if strings.Contains(u, c) {
			return true
		}
	}
	return false
}

// Empty reports whether the list has no patterns (ignores nobody).
func (l *IgnoreList) Empty() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return !l.all && len(l.exact) == 0 && len(l.prefixes) == 0 &&
		len(l.suffixes) == 0 && len(l.contains) == 0
}
