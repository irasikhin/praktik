package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTreeHashDeterministicAndOrderIndependent(t *testing.T) {
	a := t.TempDir()
	writeTree(t, a, map[string]string{"SKILL.md": "x", "ref/a.md": "aa", "ref/b.md": "bb"})
	b := t.TempDir()
	// Same content, written in a different order — hash must match.
	writeTree(t, b, map[string]string{"ref/b.md": "bb", "SKILL.md": "x", "ref/a.md": "aa"})

	ha, err := TreeHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := TreeHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Errorf("tree hashes differ for identical trees: %s vs %s", ha, hb)
	}
	// Stable across calls.
	if again, _ := TreeHash(a); again != ha {
		t.Error("tree hash not stable across calls")
	}
}

func TestTreeHashChangesOnEdit(t *testing.T) {
	d := t.TempDir()
	writeTree(t, d, map[string]string{"SKILL.md": "x", "ref/a.md": "aa"})
	before, _ := TreeHash(d)
	if err := os.WriteFile(filepath.Join(d, "ref", "a.md"), []byte("CHANGED"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := TreeHash(d)
	if before == after {
		t.Error("tree hash unchanged after editing a supporting file")
	}
}

func TestTreeHashSkipsGit(t *testing.T) {
	d := t.TempDir()
	writeTree(t, d, map[string]string{"SKILL.md": "x"})
	h1, _ := TreeHash(d)
	writeTree(t, d, map[string]string{".git/config": "junk", ".git/HEAD": "ref"})
	h2, _ := TreeHash(d)
	if h1 != h2 {
		t.Error("tree hash changed when .git content was added (must be skipped)")
	}
}

func TestParseDir(t *testing.T) {
	d := filepath.Join(t.TempDir(), "lang-go-y")
	writeTree(t, d, map[string]string{
		"SKILL.md":      "---\nname: lang-go-y\ndescription: Go style for a project here.\nmetadata: { axis: lang-go }\n---\n# Go\nbody\n",
		"reference.md":  "supporting file\n",
		"scripts/go.sh": "echo hi\n",
	})
	s, err := ParseDir(d, "skills/lang-go-y")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "lang-go-y" || s.RelDir != "skills/lang-go-y" || s.AbsDir != d {
		t.Errorf("dir provenance wrong: name=%q rel=%q abs=%q", s.Name, s.RelDir, s.AbsDir)
	}
	if !s.Valid() {
		t.Errorf("expected valid, got issues: %v", s.Issues())
	}
	// Hash is the tree hash (covers supporting files), not sha256 of SKILL.md alone.
	tree, _ := TreeHash(d)
	if s.Hash != tree {
		t.Errorf("Hash = %q, want tree hash %q", s.Hash, tree)
	}
	if s.Hash == hashOf(s.Raw) {
		t.Error("dir Hash must differ from the lone-SKILL.md hash when there are supporting files")
	}
}

func TestValidIssues(t *testing.T) {
	good := Parse("proc-x", []byte("---\nname: proc-x\ndescription: A valid process rule here.\nmetadata: { axis: proc }\n---\nbody\n"))
	if !good.Valid() {
		t.Errorf("good skill invalid: %v", good.Issues())
	}
	cases := map[string]string{
		"no frontmatter\n": "missing or unterminated",
		"---\nname: Bad_Name\ndescription: long enough description text.\n---\n": "kebab",
		"---\nname: other\ndescription: long enough description text.\n---\n":    "!= directory",
		"---\nname: proc-x\ndescription: short\n---\n":                           "description length",
	}
	for content, want := range cases {
		s := Parse("proc-x", []byte(content))
		issues := s.Issues()
		found := false
		for _, m := range issues {
			if strings.Contains(m, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("content %q: issues %v missing %q", content, issues, want)
		}
	}
}
