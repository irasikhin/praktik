package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irasikhin/praktik/internal/library"
	"github.com/irasikhin/praktik/internal/project"
	"github.com/irasikhin/praktik/internal/skill"
)

func skillMD(name, axis string) string {
	return "---\nname: " + name + "\ndescription: A skill described for tests here.\nmetadata: { axis: " + axis + " }\n---\n# " + name + "\nbody\n"
}

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

// srcOf builds a source repo from files and returns a ResolvedSource whose chosen
// skills are the discovered ones (optionally filtered to include).
func srcOf(t *testing.T, key string, files map[string]string, include ...string) ResolvedSource {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	ck, err := library.ResolveCheckout(root, "")
	if err != nil {
		t.Fatal(err)
	}
	disc, err := ck.Discover("")
	if err != nil {
		t.Fatal(err)
	}
	var chosen []*skill.Skill
	for _, s := range disc {
		if len(include) == 0 || contains(include, s.Name) {
			chosen = append(chosen, s)
		}
	}
	return ResolvedSource{Key: key, Repo: key, Commit: ck.Commit(), Skills: chosen}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func emptyLock() *project.Lock {
	return &project.Lock{Skills: map[string]project.LockedSkill{}}
}

func req(root string, lock *project.Lock, sources ...ResolvedSource) Request {
	return Request{
		Sources: sources,
		Targets: map[string]string{"claude": ".claude/skills"},
		Root:    root,
		Lock:    lock,
	}
}

func mustPlan(t *testing.T, r Request) *Plan {
	t.Helper()
	p, err := Compute(r)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return p
}

func kindOf(p *Plan, name string) ActionKind {
	for _, a := range p.Actions {
		if a.Skill == name && a.Target == "claude" {
			return a.Kind
		}
	}
	return ""
}

// destOf is the vendored skill DIRECTORY in the default "claude" target.
func destOf(root, name string) string {
	return filepath.Join(root, ".claude/skills", name)
}

// twoSkillSource is a single source providing proc-x (with a supporting file) and
// lang-go-y.
func twoSkillSource(t *testing.T, include ...string) ResolvedSource {
	return srcOf(t, "s", map[string]string{
		"skills/proc-x/SKILL.md":    skillMD("proc-x", "proc"),
		"skills/proc-x/ref.md":      "supporting file\n",
		"skills/lang-go-y/SKILL.md": skillMD("lang-go-y", "lang-go"),
	}, include...)
}
