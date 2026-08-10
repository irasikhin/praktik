package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initProject creates a temp library (skills proc-x, lang-go-y) and a fresh temp
// project, chdirs into the project, and returns (libRoot, projDir). Subsequent
// runProj calls operate on that project with the library as --source.
func initProject(t *testing.T) (lib, proj string) {
	t.Helper()
	lib = fixture(t)
	proj = t.TempDir()
	t.Chdir(proj)
	return lib, proj
}

// runProj runs the CLI against an already-set cwd, with lib as --source.
func runProj(t *testing.T, lib string, args ...string) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	full := append([]string{"--source", lib, "--no-color"}, args...)
	code := Run(full, strings.NewReader(""), &out, &errw)
	return code, out.String(), errw.String()
}

func vendored(proj, target, name string) string {
	return filepath.Join(proj, target, name, "SKILL.md")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestInitVendorsRealFiles(t *testing.T) {
	lib, proj := initProject(t)
	code, out, errw := runProj(t, lib, "init", "proc-x")
	if code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}
	if !strings.Contains(out, "created") || !strings.Contains(out, "praktik.toml") {
		t.Errorf("out = %q", out)
	}
	if _, err := os.Stat(filepath.Join(proj, "praktik.toml")); err != nil {
		t.Errorf("praktik.toml missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj, ".praktik", "lock.json")); err != nil {
		t.Errorf("lock.json missing: %v", err)
	}
	dest := vendored(proj, ".claude/skills", "proc-x")
	fi, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("vendored file missing: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("vendored file is a symlink, want a real copy")
	}
}

func TestInitRejectsExisting(t *testing.T) {
	lib, _ := initProject(t)
	if code, _, _ := runProj(t, lib, "init"); code != 0 {
		t.Fatalf("first init exit=%d", code)
	}
	code, _, errw := runProj(t, lib, "init")
	if code != 1 || !strings.Contains(errw, "already exists") {
		t.Errorf("second init: exit=%d err=%q", code, errw)
	}
}

func TestStatusNoProject(t *testing.T) {
	lib := fixture(t)
	t.Chdir(t.TempDir())
	code, _, errw := runProj(t, lib, "status")
	if code != 1 || !strings.Contains(errw, "no praktik.toml") {
		t.Errorf("status without project: exit=%d err=%q", code, errw)
	}
}

func TestAddRmLifecycle(t *testing.T) {
	lib, proj := initProject(t)
	runProj(t, lib, "init", "proc-x")

	// add a second skill → status clean afterwards (add syncs).
	if code, out, errw := runProj(t, lib, "add", "lang-go-y"); code != 0 {
		t.Fatalf("add exit=%d out=%q err=%q", code, out, errw)
	}
	if !fileExists(vendored(proj, ".claude/skills", "lang-go-y")) {
		t.Error("lang-go-y not vendored after add")
	}
	if _, out, _ := runProj(t, lib, "status"); !strings.Contains(out, "=ok") {
		t.Errorf("status after add not clean: %q", out)
	}

	// rm proc-x → its copy is pruned, lang-go-y stays.
	if code, _, errw := runProj(t, lib, "rm", "proc-x"); code != 0 {
		t.Fatalf("rm exit=%d err=%q", code, errw)
	}
	if fileExists(vendored(proj, ".claude/skills", "proc-x")) {
		t.Error("proc-x not pruned after rm")
	}
	if !fileExists(vendored(proj, ".claude/skills", "lang-go-y")) {
		t.Error("lang-go-y wrongly pruned")
	}

	// idempotent sync.
	if _, out, _ := runProj(t, lib, "sync"); !strings.Contains(out, "=ok") {
		t.Errorf("second sync not a noop: %q", out)
	}
}

func TestDriftDiffAndRestore(t *testing.T) {
	lib, proj := initProject(t)
	runProj(t, lib, "init", "proc-x")
	dest := vendored(proj, ".claude/skills", "proc-x")

	// Tamper the vendored copy.
	if err := os.WriteFile(dest, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, out, _ := runProj(t, lib, "status"); !strings.Contains(out, "~") || !strings.Contains(out, "proc-x") {
		t.Errorf("status after tamper = %q, want ~changed proc-x", out)
	}
	if _, out, _ := runProj(t, lib, "diff", "proc-x"); !strings.Contains(out, "tampered") {
		t.Errorf("diff = %q, want it to show the tampered line", out)
	}

	// sync restores byte-for-byte from source.
	if code, _, errw := runProj(t, lib, "sync"); code != 0 {
		t.Fatalf("sync exit=%d err=%q", code, errw)
	}
	got, _ := os.ReadFile(dest)
	want, _ := os.ReadFile(filepath.Join(lib, "skills", "proc-x", "SKILL.md"))
	if string(got) != string(want) {
		t.Error("sync did not restore the vendored copy to source")
	}
	if _, out, _ := runProj(t, lib, "status"); !strings.Contains(out, "=ok") {
		t.Errorf("status after restore not clean: %q", out)
	}
}

func TestTargetAddRm(t *testing.T) {
	lib, proj := initProject(t)
	runProj(t, lib, "init", "proc-x")

	if code, _, errw := runProj(t, lib, "target", "add", "codex", ".codex/skills"); code != 0 {
		t.Fatalf("target add exit=%d err=%q", code, errw)
	}
	if !fileExists(vendored(proj, ".codex/skills", "proc-x")) {
		t.Error("proc-x not vendored into the new codex target")
	}
	if _, out, _ := runProj(t, lib, "target", "ls"); !strings.Contains(out, "codex") || !strings.Contains(out, "claude") {
		t.Errorf("target ls = %q", out)
	}

	// rm codex → its copies are pruned, claude survives.
	if code, _, errw := runProj(t, lib, "target", "rm", "codex"); code != 0 {
		t.Fatalf("target rm exit=%d err=%q", code, errw)
	}
	if fileExists(vendored(proj, ".codex/skills", "proc-x")) {
		t.Error("codex copies not pruned on target rm")
	}
	if !fileExists(vendored(proj, ".claude/skills", "proc-x")) {
		t.Error("claude copy wrongly removed")
	}

	// removing the last target is refused.
	if code, _, errw := runProj(t, lib, "target", "rm", "claude"); code != 1 || !strings.Contains(errw, "last target") {
		t.Errorf("target rm last: exit=%d err=%q", code, errw)
	}
}

func TestGlobalScope(t *testing.T) {
	lib := fixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	if code, _, errw := runProj(t, lib, "-g", "init", "proc-x"); code != 0 {
		t.Fatalf("global init exit=%d err=%s", code, errw)
	}
	if !fileExists(filepath.Join(home, ".config", "praktik", "config.toml")) {
		t.Error("global config.toml not created")
	}
	if !fileExists(filepath.Join(home, ".local", "state", "praktik", "lock.json")) {
		t.Error("global state lock not created")
	}
	if !fileExists(filepath.Join(home, ".claude", "skills", "proc-x", "SKILL.md")) {
		t.Error("skill not vendored into $HOME/.claude/skills")
	}
	if _, out, _ := runProj(t, lib, "-g", "status"); !strings.Contains(out, "=ok") {
		t.Errorf("global status not clean: %q", out)
	}
}

func TestStatusJSON(t *testing.T) {
	lib, proj := initProject(t)
	runProj(t, lib, "init", "proc-x")

	if _, out, _ := runProj(t, lib, "--json", "status"); !strings.Contains(out, `"clean": true`) {
		t.Errorf("clean json status = %q", out)
	}

	// Tamper → not clean, with a changed item.
	if err := os.WriteFile(vendored(proj, ".claude/skills", "proc-x"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, _ := runProj(t, lib, "--json", "status")
	if !strings.Contains(out, `"clean": false`) || !strings.Contains(out, `"state": "changed"`) {
		t.Errorf("dirty json status = %q", out)
	}
}
