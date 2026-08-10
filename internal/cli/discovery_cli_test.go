package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/praktik/internal/picker"
)

func TestSearchCmd(t *testing.T) {
	code, out, _ := run(t, fixture(t), "search", "go")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "lang-go-y") || strings.Contains(out, "proc-x") {
		t.Errorf("search go = %q, want only lang-go-y", out)
	}
	if !strings.Contains(out, "1 match") {
		t.Errorf("missing count: %q", out)
	}
}

func TestSearchJSON(t *testing.T) {
	code, out, _ := run(t, fixture(t), "--json", "search", "go")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, `"name": "lang-go-y"`) {
		t.Errorf("json = %q", out)
	}
}

func TestPickShowsViaHook(t *testing.T) {
	picker.Hook = func(lines []string, query string) (string, error) { return "proc-x\tsummary", nil }
	t.Cleanup(func() { picker.Hook = nil })
	code, out, _ := run(t, fixture(t), "pick")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "proc-x") || !strings.Contains(out, "body text") {
		t.Errorf("pick show = %q", out)
	}
}

func TestPickAddViaHook(t *testing.T) {
	picker.Hook = func(lines []string, query string) (string, error) { return "lang-go-y\tsummary", nil }
	t.Cleanup(func() { picker.Hook = nil })
	lib, proj := initProject(t)
	runProj(t, lib, "init", "proc-x")
	if code, _, errw := runProj(t, lib, "pick", "--add"); code != 0 {
		t.Fatalf("pick --add exit=%d err=%s", code, errw)
	}
	if !fileExists(vendored(proj, ".claude/skills", "lang-go-y")) {
		t.Error("pick --add did not vendor the chosen skill")
	}
}

func TestDoctorClean(t *testing.T) {
	lib, _ := initProject(t)
	runProj(t, lib, "init", "proc-x")
	code, out, _ := runProj(t, lib, "doctor")
	if code != 0 {
		t.Fatalf("doctor exit=%d out=%s", code, out)
	}
	if !strings.Contains(out, "resolves:") || !strings.Contains(out, "in sync") {
		t.Errorf("doctor out = %q", out)
	}
}

func TestDoctorFlagsDriftAndLeftover(t *testing.T) {
	lib, proj := initProject(t)
	runProj(t, lib, "init", "proc-x")
	// Drift.
	if err := os.WriteFile(vendored(proj, ".claude/skills", "proc-x"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Leftover skillshare dir.
	if err := os.MkdirAll(filepath.Join(proj, ".skillshare"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, out, _ := runProj(t, lib, "doctor")
	if !strings.Contains(out, "out of sync") {
		t.Errorf("doctor should flag drift: %q", out)
	}
	if !strings.Contains(out, ".skillshare") {
		t.Errorf("doctor should flag leftover .skillshare: %q", out)
	}
}
