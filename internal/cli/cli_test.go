package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run drives the CLI in-process against a temp library, returning exit code,
// stdout, and stderr.
func run(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	full := append([]string{"--source", root, "--no-color"}, args...)
	code := Run(full, strings.NewReader(""), &out, &errw)
	return code, out.String(), errw.String()
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	skills := map[string]string{
		"proc-x":    "---\nname: proc-x\ndescription: A process rule for projects here.\nmetadata: { axis: proc }\n---\n# Proc\nbody text\n",
		"lang-go-y": "---\nname: lang-go-y\ndescription: Go style guidance for a project.\nmetadata: { axis: lang-go }\n---\n# Go\nbody\n",
	}
	for name, content := range skills {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestValidateCmdOK(t *testing.T) {
	code, out, _ := run(t, fixture(t), "validate")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "OK: 2 skills valid") {
		t.Errorf("out = %q", out)
	}
}

func TestValidateCmdFailureExitsNonZero(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "bad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errw := run(t, root, "validate")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errw, "FAILED") {
		t.Errorf("stderr = %q", errw)
	}
}

func TestListCmd(t *testing.T) {
	code, out, _ := run(t, fixture(t), "list")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"proc-x", "lang-go-y", "Process", "Language", "2 skills"} {
		if !strings.Contains(out, want) {
			t.Errorf("list out missing %q:\n%s", want, out)
		}
	}
}

func TestListCmdJSON(t *testing.T) {
	code, out, _ := run(t, fixture(t), "--json", "list")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, `"name": "proc-x"`) || !strings.Contains(out, `"group": "lang"`) {
		t.Errorf("json out = %s", out)
	}
}

func TestShowCmd(t *testing.T) {
	code, out, _ := run(t, fixture(t), "show", "proc-x")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "proc-x") || !strings.Contains(out, "body text") {
		t.Errorf("show out = %q", out)
	}
}

func TestShowMissingExitsNonZero(t *testing.T) {
	code, _, errw := run(t, fixture(t), "show", "nope")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errw, "not found") {
		t.Errorf("stderr = %q", errw)
	}
}

func TestHandbookWriteThenCheck(t *testing.T) {
	root := fixture(t)
	// Write.
	code, out, _ := run(t, root, "handbook")
	if code != 0 {
		t.Fatalf("write exit = %d", code)
	}
	if !strings.Contains(out, "wrote HANDBOOK.md") {
		t.Errorf("out = %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "HANDBOOK.md")); err != nil {
		t.Fatalf("HANDBOOK.md not written: %v", err)
	}
	// Check passes immediately after a write.
	code, _, _ = run(t, root, "handbook", "--check")
	if code != 0 {
		t.Fatalf("check-after-write exit = %d, want 0", code)
	}
	// Mutate → stale.
	if err := os.WriteFile(filepath.Join(root, "HANDBOOK.md"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errw := run(t, root, "handbook", "--check")
	if code != 1 {
		t.Fatalf("check-stale exit = %d, want 1", code)
	}
	if !strings.Contains(errw, "stale") {
		t.Errorf("stderr = %q", errw)
	}
}
