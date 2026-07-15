package remnawave

import (
	"os"
	"path/filepath"
	"testing"
)

func writeIgnoreFile(t *testing.T, lines string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "ignored-users.txt")
	if err := os.WriteFile(p, []byte(lines), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestIgnoreListMatch(t *testing.T) {
	p := writeIgnoreFile(t, `
# comment
tech_user*
exact_name
*_bot
*service*
`)
	l := NewIgnoreList(p)

	cases := []struct {
		name string
		want bool
	}{
		{"tech_user_1", true},   // prefix
		{"TECH_USER_X", true},   // prefix, case-insensitive
		{"tech_user", true},     // prefix boundary
		{"exact_name", true},    // exact
		{"exact_names", false},  // exact must be full
		{"telegram_bot", true},  // suffix
		{"my_service_acct", true}, // contains
		{"realcustomer", false},
		{"", false}, // empty never matches
	}
	for _, c := range cases {
		if got := l.Match(c.name); got != c.want {
			t.Errorf("Match(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIgnoreListEmptyAndMissing(t *testing.T) {
	if l := NewIgnoreList(""); !l.Empty() || l.Match("anyone") {
		t.Error("empty path should ignore nobody")
	}
	if l := NewIgnoreList("/nonexistent/path/xyz.txt"); !l.Empty() || l.Match("anyone") {
		t.Error("missing file should ignore nobody (fail-open)")
	}
}

func TestIgnoreListWildcardAll(t *testing.T) {
	p := writeIgnoreFile(t, "*\n")
	l := NewIgnoreList(p)
	if !l.Match("anyone") || l.Empty() {
		t.Error(`"*" should match everyone`)
	}
}

func TestIgnoreListReload(t *testing.T) {
	p := writeIgnoreFile(t, "# empty\n")
	l := NewIgnoreList(p)
	if l.Match("tech_user_1") {
		t.Fatal("should not match before pattern added")
	}
	if err := os.WriteFile(p, []byte("tech_user*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l.Reload()
	if !l.Match("tech_user_1") {
		t.Error("should match after reload")
	}
}
