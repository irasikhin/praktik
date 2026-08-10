package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLockRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".praktik", "lock.json")
	want := &Lock{
		Sources: []LockSource{{Name: "praktik", Repo: "r", Ref: "main", Commit: "abc1234"}},
		Skills: map[string]LockedSkill{
			"proc-x":    {Source: "praktik", Repo: "r", Ref: "main", Commit: "abc1234", Hash: "aaaa"},
			"lang-go-y": {Source: "praktik", Repo: "r", Ref: "main", Commit: "abc1234", Hash: "bbbb"},
		},
	}
	if err := SaveLock(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadLock(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lock = %+v, want %+v", got, want)
	}
}

func TestLoadLockLegacyObjectSource(t *testing.T) {
	// Old lock shape: a single `source` object + hash-only skills.
	data := `{"source":{"repo":"r","ref":"main","commit":"abc"},"skills":{"proc-x":{"hash":"aaaa"}}}`
	path := filepath.Join(t.TempDir(), ".praktik", "lock.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLock(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(l.Sources) != 1 || l.Sources[0].Repo != "r" {
		t.Errorf("legacy source not folded: %+v", l.Sources)
	}
	if l.Skills["proc-x"].Hash != "aaaa" {
		t.Errorf("legacy skill hash = %q", l.Skills["proc-x"].Hash)
	}
}

func TestLoadLockAbsentIsEmpty(t *testing.T) {
	l, err := LoadLock(filepath.Join(t.TempDir(), ".praktik", "lock.json"))
	if err != nil {
		t.Fatalf("err = %v, want nil for a missing lock", err)
	}
	if l.Skills == nil || len(l.Skills) != 0 {
		t.Errorf("skills = %+v, want an empty managed set", l.Skills)
	}
}

func TestManagedSorted(t *testing.T) {
	l := &Lock{Skills: map[string]LockedSkill{
		"zebra": {}, "alpha": {}, "mid": {},
	}}
	got := l.Managed()
	want := []string{"alpha", "mid", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Managed = %v, want %v", got, want)
	}
}

func TestSaveLockCreatesPraktikDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".praktik", "lock.json")
	if err := SaveLock(path, &Lock{Skills: map[string]LockedSkill{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf(".praktik/lock.json not created: %v", err)
	}
}
