package library

import (
	"os"
	"path/filepath"
	"testing"
)

func skillMD(name, axis string) string {
	return "---\nname: " + name + "\ndescription: A skill described for tests here.\nmetadata: { axis: " + axis + " }\n---\n# " + name + "\nbody\n"
}

// repoTree builds a temp repo dir with the given relpath→content files and
// returns a Checkout rooted at it (local, no git).
func repoTree(t *testing.T, files map[string]string) *Checkout {
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
	c, err := ResolveCheckout(root, "")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDiscoverRecursiveAndExcludes(t *testing.T) {
	c := repoTree(t, map[string]string{
		"skills/proc-x/SKILL.md":      skillMD("proc-x", "proc"),
		"tools/ml-rag/SKILL.md":       skillMD("ml-rag", "tool"),
		"deep/a/b/lang-go-y/SKILL.md": skillMD("lang-go-y", "lang-go"),
		".git/x/SKILL.md":             skillMD("x", "proc"), // excluded
		"node_modules/p/SKILL.md":     skillMD("p", "proc"), // excluded
		"bad/SKILL.md":                "no frontmatter\n",   // invalid → skipped
		"readme.md":                   "not a skill\n",
	})
	got, err := c.Discover("")
	if err != nil {
		t.Fatal(err)
	}
	var gotNames []string
	for _, s := range got {
		gotNames = append(gotNames, s.Name)
	}
	want := []string{"lang-go-y", "ml-rag", "proc-x"}
	if len(gotNames) != len(want) {
		t.Fatalf("discovered %v, want %v", gotNames, want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Errorf("discovered %v, want %v", gotNames, want)
		}
	}
	// RelDir is recorded relative to the checkout.
	for _, s := range got {
		if s.RelDir == "" || s.AbsDir == "" {
			t.Errorf("%s missing dir provenance", s.Name)
		}
	}
}

func TestDiscoverRoot(t *testing.T) {
	c := repoTree(t, map[string]string{
		"skills/proc-x/SKILL.md": skillMD("proc-x", "proc"),
		"tools/ml-rag/SKILL.md":  skillMD("ml-rag", "tool"),
	})
	got, err := c.Discover("skills")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "proc-x" {
		t.Errorf("Discover(skills) = %v, want [proc-x]", got)
	}
	if _, err := c.Discover("nope"); err == nil {
		t.Error("Discover(nope) should error on missing root")
	}
}

func TestSkillByPath(t *testing.T) {
	c := repoTree(t, map[string]string{
		"tools/ml-rag/SKILL.md": skillMD("ml-rag", "tool"),
	})
	s, err := c.SkillByPath("", "tools/ml-rag")
	if err != nil || s.Name != "ml-rag" {
		t.Fatalf("SkillByPath = %v, %v", s, err)
	}
	if _, err := c.SkillByPath("", "../escape"); err == nil {
		t.Error("path escape should error")
	}
	if _, err := c.SkillByPath("", "tools/nope"); err == nil {
		t.Error("missing path should error")
	}
}

func TestSelect(t *testing.T) {
	c := repoTree(t, map[string]string{
		"skills/proc-x/SKILL.md":    skillMD("proc-x", "proc"),
		"skills/proc-y/SKILL.md":    skillMD("proc-y", "proc"),
		"skills/lang-go-z/SKILL.md": skillMD("lang-go-z", "lang-go"),
		"tools/ml-rag/SKILL.md":     skillMD("ml-rag", "tool"),
	})
	disc, _ := c.Discover("")
	chosen, missing, err := c.Select("", disc, []string{"proc", "path:tools/ml-rag", "ghost-*"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range chosen {
		got = append(got, s.Name)
	}
	want := []string{"ml-rag", "proc-x", "proc-y"} // proc axis + the path: skill
	if len(got) != len(want) {
		t.Fatalf("chosen %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chosen %v, want %v", got, want)
		}
	}
	if len(missing) != 1 || missing[0] != "ghost-*" {
		t.Errorf("missing = %v, want [ghost-*]", missing)
	}
}

func TestResolveCheckoutGit(t *testing.T) {
	src, _ := gitBareRepoWithSkill(t, "proc-git")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	c, err := ResolveCheckout(src, "main")
	if err != nil {
		t.Fatal(err)
	}
	if c.Commit() == "" || c.Commit() == "local" {
		t.Errorf("commit = %q, want a real sha", c.Commit())
	}
	got, err := c.Discover("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "proc-git" {
		t.Errorf("discovered %v, want [proc-git]", got)
	}
}
