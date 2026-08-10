package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/praktik/internal/library"
)

func libWith(t *testing.T, skills map[string]string) *library.Library {
	t.Helper()
	root := t.TempDir()
	for name, content := range skills {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if content != "" {
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	lib, err := library.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func front(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\nmetadata: { axis: proc }\n---\nbody\n"
}

func TestRunAllValid(t *testing.T) {
	lib := libWith(t, map[string]string{
		"proc-x":    front("proc-x", "a sufficiently long description"),
		"lang-go-y": front("lang-go-y", "another good description here"),
	})
	issues, n, err := Run(lib)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("checked = %d, want 2", n)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestRunFindsEachFailure(t *testing.T) {
	lib := libWith(t, map[string]string{
		"missing-md":    "",                                               // no SKILL.md
		"no-front":      "just text, no fence\n",                          // missing frontmatter
		"Bad_Name":      front("Bad_Name", "a long enough description x"), // not kebab + name!=dir is moot (matches dir but not kebab)
		"name-mismatch": front("other-name", "a long enough description"), // name != dir
		"short":         front("short", "tiny"),                           // description too short
		"no-desc":       "---\nname: no-desc\n---\nbody\n",                // no description
	})
	issues, _, err := Run(lib)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, is := range issues {
		joined += is.Skill + ": " + is.Message + "\n"
	}
	wants := []string{
		"missing-md: missing SKILL.md",
		"no-front: missing or unterminated",
		"Bad_Name: name 'Bad_Name' is not kebab-case",
		"name-mismatch: name 'other-name' != directory 'name-mismatch'",
		"short: description length 4 outside [10,1024]",
		"no-desc: frontmatter has no `description:`",
	}
	for _, w := range wants {
		if !strings.Contains(joined, w) {
			t.Errorf("missing expected issue %q in:\n%s", w, joined)
		}
	}
}
