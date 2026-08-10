package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skillMDc(name, axis string) string {
	return "---\nname: " + name + "\ndescription: A skill described for tests here.\nmetadata: { axis: " + axis + " }\n---\n# " + name + "\nbody\n"
}

// localSkillRepo builds a temp source repo (local dir) with the given files.
func localSkillRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runBare runs the CLI in the current dir with no --source (sources come from the
// manifest's explicit repos).
func runBare(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	full := append([]string{"--no-color"}, args...)
	code := Run(full, strings.NewReader(""), &out, &errw)
	return code, out.String(), errw.String()
}

func TestSourceAddRmLifecycle(t *testing.T) {
	a := localSkillRepo(t, map[string]string{"skills/proc-x/SKILL.md": skillMDc("proc-x", "proc")})
	b := localSkillRepo(t, map[string]string{
		"tools/ml-rag/SKILL.md": skillMDc("ml-rag", "tool"),
		"tools/ml-rag/ref.md":   "supporting\n",
	})
	proj := t.TempDir()
	t.Chdir(proj)

	// init pins source A (via --source, persisted into the manifest).
	if code, _, errw := runProj(t, a, "init", "proc"); code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}
	// add a second source B selecting a skill by path.
	if code, _, errw := runBare(t, "source", "add", b, "path:tools/ml-rag"); code != 0 {
		t.Fatalf("source add exit=%d err=%s", code, errw)
	}
	if !fileExists(filepath.Join(proj, ".claude/skills/proc-x/SKILL.md")) {
		t.Error("proc-x (source A) not vendored")
	}
	if !fileExists(filepath.Join(proj, ".claude/skills/ml-rag/ref.md")) {
		t.Error("ml-rag (source B) not vendored as a whole dir")
	}
	if _, out, _ := runBare(t, "status"); !strings.Contains(out, "=ok") {
		t.Errorf("status not clean: %q", out)
	}
	if _, out, _ := runBare(t, "source", "ls"); !strings.Contains(out, b) {
		t.Errorf("source ls missing B: %q", out)
	}

	// rm source B → ml-rag pruned, proc-x stays.
	if code, _, errw := runBare(t, "source", "rm", b); code != 0 {
		t.Fatalf("source rm exit=%d err=%s", code, errw)
	}
	if fileExists(filepath.Join(proj, ".claude/skills/ml-rag")) {
		t.Error("ml-rag not pruned after source rm")
	}
	if !fileExists(filepath.Join(proj, ".claude/skills/proc-x/SKILL.md")) {
		t.Error("proc-x wrongly pruned")
	}
}

func TestCLICollisionFailsLoud(t *testing.T) {
	a := localSkillRepo(t, map[string]string{"skills/dup/SKILL.md": skillMDc("dup", "proc")})
	b := localSkillRepo(t, map[string]string{"x/dup/SKILL.md": skillMDc("dup", "proc")})
	proj := t.TempDir()
	t.Chdir(proj)
	runProj(t, a, "init", "dup")
	// Adding B (also providing dup) makes sync fail loud.
	code, _, errw := runBare(t, "source", "add", b, "*")
	if code != 1 || !strings.Contains(errw, "provided by both") {
		t.Errorf("collision: exit=%d err=%q", code, errw)
	}
}

func TestLegacyManifestRunsThroughCLI(t *testing.T) {
	a := localSkillRepo(t, map[string]string{"skills/proc-x/SKILL.md": skillMDc("proc-x", "proc")})
	proj := t.TempDir()
	t.Chdir(proj)
	legacy := "skills = [\"proc\"]\n\n[source]\n  repo = \"" + a + "\"\n\n[targets]\n  claude = \".claude/skills\"\n"
	if err := os.WriteFile(filepath.Join(proj, "praktik.toml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errw := runBare(t, "sync"); code != 0 {
		t.Fatalf("legacy sync exit=%d err=%s", code, errw)
	}
	if !fileExists(filepath.Join(proj, ".claude/skills/proc-x/SKILL.md")) {
		t.Error("legacy manifest did not vendor proc-x")
	}
}
