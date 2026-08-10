package library

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// repo writes skills under a temp repo and returns its root.
func repo(t *testing.T, skills map[string]string) string {
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
	return root
}

func skillFile(name, axis string) string {
	return "---\nname: " + name + "\ndescription: a description long enough\nmetadata: { axis: " + axis + " }\n---\nbody\n"
}

func TestResolveOverride(t *testing.T) {
	root := repo(t, map[string]string{"proc-x": skillFile("proc-x", "proc")})

	// repo root → finds skills/
	lib, err := Resolve(root)
	if err != nil {
		t.Fatalf("Resolve(repo) error: %v", err)
	}
	if lib.Dir() != filepath.Join(root, "skills") {
		t.Errorf("Dir = %q", lib.Dir())
	}

	// the skills dir itself → also resolves
	if _, err := Resolve(filepath.Join(root, "skills")); err != nil {
		t.Errorf("Resolve(skillsDir) error: %v", err)
	}

	// a bogus path → error
	if _, err := Resolve(filepath.Join(root, "nope")); err == nil {
		t.Error("expected error for bogus override")
	}
}

func TestSkillsAndNames(t *testing.T) {
	root := repo(t, map[string]string{
		"proc-x":     skillFile("proc-x", "proc"),
		"lang-go-y":  skillFile("lang-go-y", "lang-go"),
		"no-skillmd": "", // directory with no SKILL.md
	})
	lib, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	names, err := lib.Names()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"lang-go-y", "proc-x"}) {
		t.Errorf("Names = %v (dirs without SKILL.md must be skipped)", names)
	}

	// ReadSkill on the empty dir → ok == false.
	if _, ok, err := lib.ReadSkill("no-skillmd"); err != nil || ok {
		t.Errorf("ReadSkill(no-skillmd) = ok %v err %v", ok, err)
	}

	if _, err := lib.Skill("does-not-exist"); err == nil {
		t.Error("Skill(missing) should error")
	}
}

func TestExpand(t *testing.T) {
	all := []string{"lang-go-style", "lang-go-test", "proc-commits", "praktik-index", "tool-sops", "tool-sops-extra"}
	cases := []struct {
		pats []string
		want []string
	}{
		{[]string{"lang-go"}, []string{"lang-go-style", "lang-go-test"}},
		{[]string{"lang"}, []string{"lang-go-style", "lang-go-test"}},
		{[]string{"proc-commits"}, []string{"proc-commits"}},
		{[]string{"praktik-index"}, []string{"praktik-index"}},
		{[]string{"*index"}, []string{"praktik-index"}},
		{[]string{"lang-go", "proc"}, []string{"lang-go-style", "lang-go-test", "proc-commits"}},
		{[]string{"lang-go", "lang-go-style"}, []string{"lang-go-style", "lang-go-test"}}, // dedup
		{[]string{"nomatch"}, []string{}},
		{[]string{"tool-sops"}, []string{"tool-sops"}},                     // exact wins over prefix
		{[]string{"tool"}, []string{"tool-sops", "tool-sops-extra"}},       // axis token still expands
		{[]string{"tool-sops*"}, []string{"tool-sops", "tool-sops-extra"}}, // glob opts back in
	}
	for _, tc := range cases {
		got := Expand(all, tc.pats)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Expand(%v) = %v, want %v", tc.pats, got, tc.want)
		}
	}
}
