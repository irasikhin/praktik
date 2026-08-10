package handbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irasikhin/praktik/internal/library"
)

func TestSummarize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"A thing. Use when committing code.", "A thing."},
		{"A thing.  Use whenever you commit.", "A thing."},
		{"First sentence. Second sentence.", "First sentence."},
		{"No trigger and no period", "No trigger and no period"},
		{"Trailing dots... Use when x.", "Trailing dots."},
	}
	for _, tc := range cases {
		if got := Summarize(tc.in); got != tc.want {
			t.Errorf("Summarize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGroup(t *testing.T) {
	cases := map[string]string{
		"proc":      "proc",
		"index":     "index",
		"ci":        "ci",
		"lang-java": "lang",
		"build-nix": "build",
		"weird":     "other",
	}
	for axis, want := range cases {
		if got := Group(axis); got != want {
			t.Errorf("Group(%q) = %q, want %q", axis, got, want)
		}
	}
}

func skillFile(name, axis, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\nmetadata: { axis: " + axis + " }\n---\nbody\n"
}

func TestGenerate(t *testing.T) {
	root := t.TempDir()
	skills := map[string]string{
		"praktik-index": skillFile("praktik-index", "index", "The umbrella index. Use when starting."),
		"proc-x":        skillFile("proc-x", "proc", "A process rule. Use when committing."),
		"lang-go-y":     skillFile("lang-go-y", "lang-go", "Go style. Use when writing Go."),
		"no-md":         "", // skipped
	}
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
	out, n, err := Generate(lib)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("count = %d, want 3", n)
	}
	for _, want := range []string{
		"# praktik handbook\n",
		"_3 skills across 3 axes._",
		"## Index",
		"## Process — every project",
		"## Language",
		"- **[proc-x](skills/proc-x/SKILL.md)** — A process rule.",
		"- **[lang-go-y](skills/lang-go-y/SKILL.md)** — Go style.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
	// Index group must render before Process before Language.
	iIndex := strings.Index(out, "## Index")
	iProc := strings.Index(out, "## Process")
	iLang := strings.Index(out, "## Language")
	if iIndex >= iProc || iProc >= iLang {
		t.Errorf("group order wrong: index=%d proc=%d lang=%d", iIndex, iProc, iLang)
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Error("output must end with exactly one trailing newline")
	}
}
