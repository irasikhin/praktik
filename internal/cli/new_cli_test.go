package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScaffoldsValidSkill(t *testing.T) {
	lib := fixture(t)
	code, out, errw := run(t, lib, "new", "lang-rust-style", "-d", "Rust style conventions for a project here.")
	if code != 0 {
		t.Fatalf("new exit=%d err=%s", code, errw)
	}
	path := filepath.Join(lib, "skills", "lang-rust-style", "SKILL.md")
	if !strings.Contains(out, path) {
		t.Errorf("out = %q, want the created path", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("scaffolded file missing: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "name: lang-rust-style") || !strings.Contains(body, "axis: lang-rust") {
		t.Errorf("frontmatter wrong:\n%s", body)
	}

	// The scaffold must pass the validator.
	if code, _, _ := run(t, lib, "validate"); code != 0 {
		t.Errorf("scaffolded skill fails validate (exit=%d)", code)
	}
}

func TestNewRejectsBadNameAndDuplicate(t *testing.T) {
	lib := fixture(t)
	if code, _, errw := run(t, lib, "new", "Bad_Name"); code != 1 || !strings.Contains(errw, "kebab") {
		t.Errorf("bad name: exit=%d err=%q", code, errw)
	}
	// proc-x already exists in the fixture.
	if code, _, errw := run(t, lib, "new", "proc-x"); code != 1 || !strings.Contains(errw, "already exists") {
		t.Errorf("duplicate: exit=%d err=%q", code, errw)
	}
}

func TestInferAxis(t *testing.T) {
	cases := map[string]string{
		"lang-java-naming":  "lang-java",
		"build-maven-quick": "build-maven",
		"proc-doc-as-code":  "proc",
		"flow-root-cause":   "flow",
		"index":             "index",
	}
	for name, want := range cases {
		if got := inferAxis(name); got != want {
			t.Errorf("inferAxis(%q) = %q, want %q", name, got, want)
		}
	}
}
