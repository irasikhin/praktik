package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irasikhin/praktik/internal/project"
)

func TestApplyWritesRealDir(t *testing.T) {
	root := t.TempDir()
	src := twoSkillSource(t, "proc-x")
	r := req(root, emptyLock(), src)
	if err := Apply(r, mustPlan(t, r), lockPathIn(root)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	dir := destOf(root, "proc-x")
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("vendored skill is not a real dir: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("vendored dir is a symlink, want a real copy")
	}
	// The whole directory is vendored, supporting files included.
	for _, f := range []string{"SKILL.md", "ref.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing vendored file %s: %v", f, err)
		}
	}
	lock, _ := project.LoadLock(lockPathIn(root))
	if lock.Skills["proc-x"].Hash != src.Skills[0].Hash {
		t.Errorf("lock hash mismatch")
	}
}

func TestApplyPrunesOnlyManaged(t *testing.T) {
	root := t.TempDir()
	r0 := req(root, emptyLock(), twoSkillSource(t, "proc-x"))
	if err := Apply(r0, mustPlan(t, r0), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	// A foreign skill the tool does not manage, in the same target.
	foreign := destOf(root, "my-own-skill")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Desire nothing: proc-x prunes, foreign survives.
	lock, _ := project.LoadLock(lockPathIn(root))
	r := req(root, lock) // no sources → empty desired
	if err := Apply(r, mustPlan(t, r), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	if dirExists(destOf(root, "proc-x")) {
		t.Error("managed proc-x not pruned")
	}
	if !dirExists(foreign) {
		t.Error("foreign skill was pruned — must never touch unmanaged dirs")
	}
}

func TestApplyDirDriftRestores(t *testing.T) {
	root := t.TempDir()
	src := twoSkillSource(t, "proc-x")
	r := req(root, emptyLock(), src)
	if err := Apply(r, mustPlan(t, r), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	dir := destOf(root, "proc-x")
	// Tamper: edit a file and add a stray one.
	if err := os.WriteFile(filepath.Join(dir, "ref.md"), []byte("TAMPERED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-apply restores the whole tree from source.
	lock, _ := project.LoadLock(lockPathIn(root))
	r2 := req(root, lock, src)
	if err := Apply(r2, mustPlan(t, r2), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	if dirExists(filepath.Join(dir, "stray.txt")) || fileExistsTest(filepath.Join(dir, "stray.txt")) {
		t.Error("stray file not removed by re-vendor")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "ref.md"))
	if string(got) != "supporting file\n" {
		t.Errorf("ref.md not restored: %q", got)
	}
}

func fileExistsTest(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
