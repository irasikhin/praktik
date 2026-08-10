package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irasikhin/praktik/internal/project"
)

func lockPathIn(root string) string { return filepath.Join(root, ".praktik", "lock.json") }

func TestPlanFreshInstall(t *testing.T) {
	root := t.TempDir()
	p := mustPlan(t, req(root, emptyLock(), twoSkillSource(t)))
	if kindOf(p, "proc-x") != Install || kindOf(p, "lang-go-y") != Install {
		t.Errorf("want both install, got proc-x=%q lang-go-y=%q", kindOf(p, "proc-x"), kindOf(p, "lang-go-y"))
	}
	if p.Count(Install) != 2 {
		t.Errorf("installs = %d, want 2", p.Count(Install))
	}
	ls := p.NextLock.Skills["proc-x"]
	if ls.Source != "s" || ls.Hash == "" {
		t.Errorf("lock entry = %+v, want source s + a hash", ls)
	}
}

func TestPlanNoopAfterApply(t *testing.T) {
	root := t.TempDir()
	src := twoSkillSource(t)
	r := req(root, emptyLock(), src)
	if err := Apply(r, mustPlan(t, r), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	lock, _ := project.LoadLock(lockPathIn(root))
	p2 := mustPlan(t, req(root, lock, src))
	if p2.Changes() {
		t.Errorf("second plan reports changes, want clean: %+v", p2.Actions)
	}
}

func TestPlanPrunesUndesired(t *testing.T) {
	root := t.TempDir()
	full := twoSkillSource(t)
	r0 := req(root, emptyLock(), full)
	if err := Apply(r0, mustPlan(t, r0), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	lock, _ := project.LoadLock(lockPathIn(root))
	// Now desire only lang-go-y.
	p := mustPlan(t, req(root, lock, twoSkillSource(t, "lang-go-y")))
	if kindOf(p, "proc-x") != Prune {
		t.Errorf("proc-x = %q, want prune", kindOf(p, "proc-x"))
	}
}

func TestPlanDetectsDirDrift(t *testing.T) {
	root := t.TempDir()
	src := twoSkillSource(t, "proc-x")
	r := req(root, emptyLock(), src)
	if err := Apply(r, mustPlan(t, r), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	// Tamper a supporting file inside the vendored dir.
	if err := os.WriteFile(filepath.Join(destOf(root, "proc-x"), "ref.md"), []byte("TAMPERED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, _ := project.LoadLock(lockPathIn(root))
	p := mustPlan(t, req(root, lock, src))
	if kindOf(p, "proc-x") != Update {
		t.Errorf("proc-x = %q, want update (dir drift)", kindOf(p, "proc-x"))
	}
}

func TestPlanCollisionFailsLoud(t *testing.T) {
	root := t.TempDir()
	a := srcOf(t, "alpha", map[string]string{"skills/dup/SKILL.md": skillMD("dup", "proc")})
	b := srcOf(t, "beta", map[string]string{"x/dup/SKILL.md": skillMD("dup", "proc")})
	_, err := Compute(req(root, emptyLock(), a, b))
	if err == nil {
		t.Fatal("expected a collision error")
	}
	var ce *CollisionError
	if !asCollision(err, &ce) || ce.Name != "dup" {
		t.Errorf("err = %v, want CollisionError for dup", err)
	}
}

func asCollision(err error, target **CollisionError) bool {
	if ce, ok := err.(*CollisionError); ok {
		*target = ce
		return true
	}
	return false
}

func TestScopeIncludes(t *testing.T) {
	cases := []struct {
		name  string
		scope Scope
		skill string
		want  bool
	}{
		{"zero value includes everything", Scope{}, "proc-x", true},
		{"named skill is in scope", ScopeTo("proc-x"), "proc-x", true},
		{"other skill is out of scope", ScopeTo("proc-x"), "lang-go-y", false},
		{"restricted-to-nothing excludes everything", ScopeTo(), "proc-x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.Includes(tc.skill); got != tc.want {
				t.Errorf("Includes(%q) = %v, want %v", tc.skill, got, tc.want)
			}
		})
	}
}

// vendorBoth applies a full two-skill reconcile and returns the resulting lock,
// the starting state for the scoped-plan tests.
func vendorBoth(t *testing.T, root string) *project.Lock {
	t.Helper()
	r := req(root, emptyLock(), twoSkillSource(t))
	if err := Apply(r, mustPlan(t, r), lockPathIn(root)); err != nil {
		t.Fatal(err)
	}
	lock, err := project.LoadLock(lockPathIn(root))
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func TestPlanScopedUpdatesOnlyInScope(t *testing.T) {
	root := t.TempDir()
	lock := vendorBoth(t, root)
	// Tamper BOTH vendored copies; only the in-scope one may be repaired.
	for _, name := range []string{"proc-x", "lang-go-y"} {
		if err := os.WriteFile(filepath.Join(destOf(root, name), "SKILL.md"), []byte("TAMPERED\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := req(root, lock, twoSkillSource(t))
	r.Scope = ScopeTo("proc-x")
	p := mustPlan(t, r)
	if kindOf(p, "proc-x") != Update {
		t.Errorf("proc-x = %q, want update", kindOf(p, "proc-x"))
	}
	if kindOf(p, "lang-go-y") != "" {
		t.Errorf("lang-go-y = %q, want no action (frozen)", kindOf(p, "lang-go-y"))
	}
	if p.NextLock.Skills["lang-go-y"] != lock.Skills["lang-go-y"] {
		t.Errorf("frozen lock entry changed: %+v -> %+v", lock.Skills["lang-go-y"], p.NextLock.Skills["lang-go-y"])
	}
}

func TestPlanScopedSkipsDesiredNewOutOfScope(t *testing.T) {
	root := t.TempDir()
	r := req(root, emptyLock(), twoSkillSource(t))
	r.Scope = ScopeTo("proc-x")
	p := mustPlan(t, r)
	if kindOf(p, "proc-x") != Install {
		t.Errorf("proc-x = %q, want install", kindOf(p, "proc-x"))
	}
	if kindOf(p, "lang-go-y") != "" {
		t.Errorf("lang-go-y = %q, want no action (out of scope)", kindOf(p, "lang-go-y"))
	}
	if _, ok := p.NextLock.Skills["lang-go-y"]; ok {
		t.Error("out-of-scope desired-but-unlocked skill must not enter the lock")
	}
}

func TestPlanScopedPrunesStaleInScope(t *testing.T) {
	root := t.TempDir()
	lock := vendorBoth(t, root)
	// proc-x is no longer desired anywhere, and the scope names it: prune it.
	r := req(root, lock, twoSkillSource(t, "lang-go-y"))
	r.Scope = ScopeTo("proc-x")
	p := mustPlan(t, r)
	if kindOf(p, "proc-x") != Prune {
		t.Errorf("proc-x = %q, want prune", kindOf(p, "proc-x"))
	}
	if _, ok := p.NextLock.Skills["proc-x"]; ok {
		t.Error("pruned skill must leave the lock")
	}
	// lang-go-y is desired but out of scope: frozen, lock entry carried over.
	if kindOf(p, "lang-go-y") != "" {
		t.Errorf("lang-go-y = %q, want no action (frozen)", kindOf(p, "lang-go-y"))
	}
	if p.NextLock.Skills["lang-go-y"] != lock.Skills["lang-go-y"] {
		t.Errorf("frozen lock entry changed: %+v -> %+v", lock.Skills["lang-go-y"], p.NextLock.Skills["lang-go-y"])
	}
}

func TestPlanScopedFreezesStaleOutOfScope(t *testing.T) {
	root := t.TempDir()
	lock := vendorBoth(t, root)
	// proc-x is stale (locked, no longer desired) but OUT of scope: no prune,
	// lock entry preserved verbatim so a later full run still manages it.
	r := req(root, lock, twoSkillSource(t, "lang-go-y"))
	r.Scope = ScopeTo("lang-go-y")
	p := mustPlan(t, r)
	if kindOf(p, "proc-x") != "" {
		t.Errorf("proc-x = %q, want no action (frozen stale)", kindOf(p, "proc-x"))
	}
	if p.NextLock.Skills["proc-x"] != lock.Skills["proc-x"] {
		t.Errorf("frozen lock entry changed: %+v -> %+v", lock.Skills["proc-x"], p.NextLock.Skills["proc-x"])
	}
	if kindOf(p, "lang-go-y") != Noop {
		t.Errorf("lang-go-y = %q, want noop", kindOf(p, "lang-go-y"))
	}
}

func TestPlanScopedPreservesMovedLockEntryVerbatim(t *testing.T) {
	root := t.TempDir()
	// The lock attributes lang-go-y to a source state that no longer exists; the
	// frozen entry must survive byte-for-byte, not be rewritten to the new source.
	stale := project.LockedSkill{Source: "olds", Repo: "old-repo", Ref: "v1", Commit: "cafe", Hash: "deadbeef"}
	lock := emptyLock()
	lock.Skills["lang-go-y"] = stale
	r := req(root, lock, twoSkillSource(t))
	r.Scope = ScopeTo("proc-x")
	p := mustPlan(t, r)
	if kindOf(p, "lang-go-y") != "" {
		t.Errorf("lang-go-y = %q, want no action (frozen)", kindOf(p, "lang-go-y"))
	}
	if p.NextLock.Skills["lang-go-y"] != stale {
		t.Errorf("lock entry = %+v, want verbatim %+v", p.NextLock.Skills["lang-go-y"], stale)
	}
}

func TestPlanScopedToNothingFreezesEverything(t *testing.T) {
	root := t.TempDir()
	lock := vendorBoth(t, root)
	if err := os.WriteFile(filepath.Join(destOf(root, "proc-x"), "ref.md"), []byte("TAMPERED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Patterns that matched nothing scope to the empty set — freeze everything
	// rather than fall back to a full reconcile.
	r := req(root, lock, twoSkillSource(t))
	r.Scope = ScopeTo()
	p := mustPlan(t, r)
	if len(p.Actions) != 0 {
		t.Errorf("actions = %+v, want none", p.Actions)
	}
	for _, name := range []string{"proc-x", "lang-go-y"} {
		if p.NextLock.Skills[name] != lock.Skills[name] {
			t.Errorf("%s lock entry changed: %+v -> %+v", name, lock.Skills[name], p.NextLock.Skills[name])
		}
	}
}
