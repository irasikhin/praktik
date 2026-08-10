package library

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitBareRepoWithSkill builds a real git repo on branch main containing one
// skill, then returns a BARE clone of it plus the working clone (so a test can
// push new commits to the bare "origin"). A bare repo has no checked-out skills/
// dir, so it exercises the clone-on-demand path (a working checkout with skills/
// would resolve as a plain local source instead).
func gitBareRepoWithSkill(t *testing.T, name string) (bare, work string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	work = filepath.Join(root, "work")
	dir := filepath.Join(work, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: A skill served over git here.\nmetadata: { axis: proc }\n---\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "init", "-b", "main")
	runGit(t, work, "config", "user.email", "t@example.com")
	runGit(t, work, "config", "user.name", "T")
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-m", "seed")

	bare = filepath.Join(root, "bare.git")
	runGit(t, "", "clone", "--bare", work, bare)
	return bare, work
}

// gitAdvance commits a new file in the working clone and pushes main to the bare
// origin, returning the new tip sha — the "upstream moved" half of the cache
// semantics tests.
func gitAdvance(t *testing.T, work, bare string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(work, "ADVANCE.md"), []byte("moved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-m", "advance")
	runGit(t, work, "push", "--quiet", bare, "main:main")
	out, err := exec.Command("git", "-C", work, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestResolveGitSource(t *testing.T) {
	src, _ := gitBareRepoWithSkill(t, "proc-git")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	lib, err := ResolveSource(src, "main")
	if err != nil {
		t.Fatalf("ResolveSource: %v", err)
	}
	if lib.Commit() == "" || lib.Commit() == "local" {
		t.Errorf("commit = %q, want a real sha", lib.Commit())
	}
	names, err := lib.Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "proc-git" {
		t.Errorf("names = %v, want [proc-git]", names)
	}

	// A second resolve hits the cached clone as-is and still works (no fetch).
	if _, err := ResolveSource(src, "main"); err != nil {
		t.Fatalf("re-resolve (cached): %v", err)
	}
}

// TestGitCacheAndRefresh pins the fetch semantics: a plain resolve never touches
// the origin once the cache exists; only a refresh-resolve does.
func TestGitCacheAndRefresh(t *testing.T) {
	bare, work := gitBareRepoWithSkill(t, "proc-git")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	// First resolve clones the cache at commit A.
	first, err := ResolveCheckout(bare, "main")
	if err != nil {
		t.Fatalf("initial ResolveCheckout: %v", err)
	}
	commitA := first.Commit()
	if commitA == "" || commitA == "local" {
		t.Fatalf("commit = %q, want a real sha", commitA)
	}

	// The origin advances to commit B.
	commitB := gitAdvance(t, work, bare)
	if commitB == commitA {
		t.Fatal("fixture did not advance the origin")
	}

	steps := []struct {
		name    string
		resolve func() (string, error)
		want    string
	}{
		{"ResolveCheckout uses the cache as-is", func() (string, error) {
			c, err := ResolveCheckout(bare, "main")
			if err != nil {
				return "", err
			}
			return c.Commit(), nil
		}, commitA},
		{"ResolveSource uses the cache as-is", func() (string, error) {
			l, err := ResolveSource(bare, "main")
			if err != nil {
				return "", err
			}
			return l.Commit(), nil
		}, commitA},
		{"RefreshCheckout fetches the new commit", func() (string, error) {
			c, err := RefreshCheckout(bare, "main")
			if err != nil {
				return "", err
			}
			return c.Commit(), nil
		}, commitB},
		{"ResolveCheckout sees the refreshed cache", func() (string, error) {
			c, err := ResolveCheckout(bare, "main")
			if err != nil {
				return "", err
			}
			return c.Commit(), nil
		}, commitB},
	}
	for _, st := range steps {
		got, err := st.resolve()
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if got != st.want {
			t.Errorf("%s: commit = %s, want %s", st.name, got, st.want)
		}
	}
}

func TestLooksRemote(t *testing.T) {
	cases := map[string]bool{
		"https://git.example.com/o/r":         true,
		"git@github.com:o/r.git":              true,
		"github.com/irasikhin/praktik-skills": true,
		"plain-name":                          false,
		"./local/path":                        false,
	}
	for in, want := range cases {
		if got := looksRemote(in); got != want {
			t.Errorf("looksRemote(%q) = %v, want %v", in, got, want)
		}
	}
}
