package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/praktik/internal/project"
)

// gitSkillSource builds a real git repo holding skills/proc-x, returning a BARE
// clone (the "remote" a manifest source clones from) plus the working clone used
// to push new commits to it.
func gitSkillSource(t *testing.T) (bare, work string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	work = filepath.Join(root, "work")
	writeSkill(t, work, "body v1")
	gitCLI(t, work, "init", "-b", "main")
	gitCLI(t, work, "config", "user.email", "t@example.com")
	gitCLI(t, work, "config", "user.name", "T")
	gitCLI(t, work, "add", "-A")
	gitCLI(t, work, "commit", "-m", "seed")
	bare = filepath.Join(root, "bare.git")
	gitCLI(t, "", "clone", "--bare", work, bare)
	return bare, work
}

func writeSkill(t *testing.T, work, body string) {
	t.Helper()
	writeNamedSkill(t, work, "proc-x", "proc", body)
}

func writeNamedSkill(t *testing.T, work, name, axis, body string) {
	t.Helper()
	dir := filepath.Join(work, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: A skill served over git for tests.\nmetadata: { axis: " + axis + " }\n---\n# " + name + "\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCLI(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestUpdateFetchesGitSourceSyncDoesNot pins the CLI-level fetch semantics: once
// a git source is cached, `praktik sync` keeps serving the cached commit, and only
// `praktik update` fetches the origin's new tip.
func TestUpdateFetchesGitSourceSyncDoesNot(t *testing.T) {
	bare, work := gitSkillSource(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	proj := t.TempDir()
	t.Chdir(proj)

	// init clones the bare source into the cache and vendors v1.
	if code, _, errw := runProj(t, bare, "init", "proc-x"); code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}
	dest := filepath.Join(proj, ".claude/skills/proc-x/SKILL.md")
	if got, _ := os.ReadFile(dest); !strings.Contains(string(got), "body v1") {
		t.Fatalf("vendored = %q, want body v1", got)
	}

	// The origin advances to v2.
	writeSkill(t, work, "body v2")
	gitCLI(t, work, "add", "-A")
	gitCLI(t, work, "commit", "-m", "v2")
	gitCLI(t, work, "push", "--quiet", bare, "main:main")

	// sync resolves from the cache as-is — still v1.
	if code, _, errw := runBare(t, "sync"); code != 0 {
		t.Fatalf("sync exit=%d err=%s", code, errw)
	}
	if got, _ := os.ReadFile(dest); !strings.Contains(string(got), "body v1") {
		t.Errorf("after sync vendored = %q, want body v1 (no fetch)", got)
	}

	// update force-fetches the source — now v2.
	if code, _, errw := runBare(t, "update"); code != 0 {
		t.Fatalf("update exit=%d err=%s", code, errw)
	}
	if got, _ := os.ReadFile(dest); !strings.Contains(string(got), "body v2") {
		t.Errorf("after update vendored = %q, want body v2 (fetched)", got)
	}
}

// TestUpdatePatternScopesReconcile pins the scoped-update semantics: with both
// vendored skills behind the source, `praktik update proc-x` refreshes only
// proc-x and freezes lang-go-y — files untouched, lock entry preserved verbatim
// — so a later bare `praktik update` still manages it and brings it up to date.
func TestUpdatePatternScopesReconcile(t *testing.T) {
	bare, work := gitSkillSource(t)
	writeNamedSkill(t, work, "lang-go-y", "lang-go", "go body v1")
	gitCLI(t, work, "add", "-A")
	gitCLI(t, work, "commit", "-m", "add lang-go-y")
	gitCLI(t, work, "push", "--quiet", bare, "main:main")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	proj := t.TempDir()
	t.Chdir(proj)

	if code, _, errw := runProj(t, bare, "init", "proc-x", "lang-go-y"); code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}
	procDest := filepath.Join(proj, ".claude/skills/proc-x/SKILL.md")
	goDest := filepath.Join(proj, ".claude/skills/lang-go-y/SKILL.md")
	lockPath := filepath.Join(proj, ".praktik", "lock.json")
	before, err := project.LoadLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	// The origin advances BOTH skills.
	writeSkill(t, work, "body v2")
	writeNamedSkill(t, work, "lang-go-y", "lang-go", "go body v2")
	gitCLI(t, work, "add", "-A")
	gitCLI(t, work, "commit", "-m", "v2")
	gitCLI(t, work, "push", "--quiet", bare, "main:main")

	// A scoped update refreshes only the named skill.
	if code, _, errw := runBare(t, "update", "proc-x"); code != 0 {
		t.Fatalf("scoped update exit=%d err=%s", code, errw)
	}
	if got, _ := os.ReadFile(procDest); !strings.Contains(string(got), "body v2") {
		t.Errorf("proc-x = %q, want body v2 (in scope)", got)
	}
	if got, _ := os.ReadFile(goDest); !strings.Contains(string(got), "go body v1") {
		t.Errorf("lang-go-y = %q, want go body v1 (frozen)", got)
	}

	// The frozen skill's lock entry survived verbatim; the updated one advanced.
	after, err := project.LoadLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Skills["lang-go-y"] != before.Skills["lang-go-y"] {
		t.Errorf("frozen lock entry changed: %+v -> %+v", before.Skills["lang-go-y"], after.Skills["lang-go-y"])
	}
	if after.Skills["proc-x"] == before.Skills["proc-x"] {
		t.Error("proc-x lock entry did not advance after scoped update")
	}

	// A pattern matching nothing warns and changes nothing.
	if code, _, errw := runBare(t, "update", "nope"); code != 0 {
		t.Fatalf("no-match update exit=%d err=%s", code, errw)
	} else if !strings.Contains(errw, `"nope" matched no desired or locked skill`) {
		t.Errorf("stderr = %q, want unmatched-pattern warning", errw)
	}
	if got, _ := os.ReadFile(goDest); !strings.Contains(string(got), "go body v1") {
		t.Errorf("lang-go-y = %q, want go body v1 (no-match update must not touch it)", got)
	}

	// A bare update still sees the frozen skill as managed and updates it.
	if code, _, errw := runBare(t, "update"); code != 0 {
		t.Fatalf("full update exit=%d err=%s", code, errw)
	}
	if got, _ := os.ReadFile(goDest); !strings.Contains(string(got), "go body v2") {
		t.Errorf("lang-go-y = %q, want go body v2 (full update)", got)
	}
}
