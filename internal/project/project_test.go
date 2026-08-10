package project

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestName)
	want := &Manifest{
		Sources: []Source{
			{Name: "praktik", Repo: "github.com/irasikhin/praktik-skills", Ref: "main", Skills: []string{"proc", "lang-go"}},
			{Repo: "github.com/o/r", Root: "skills", Skills: []string{"*", "path:tools/foo"}},
		},
		Targets: map[string]string{"claude": ".claude/skills", "codex": ".codex/skills"},
	}
	if err := SaveManifest(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got.Sources, want.Sources) {
		t.Errorf("sources = %+v, want %+v", got.Sources, want.Sources)
	}
	if !reflect.DeepEqual(got.Targets, want.Targets) {
		t.Errorf("targets = %+v, want %+v", got.Targets, want.Targets)
	}
}

func TestLoadManifestLegacy(t *testing.T) {
	// The old single-[source] + top-level skills shape folds into one Source.
	legacy := `skills = ["proc", "lang-go"]

[source]
  repo = "github.com/irasikhin/praktik-skills"
  ref = "v1"

[targets]
  claude = ".claude/skills"
`
	path := filepath.Join(t.TempDir(), ManifestName)
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("sources = %+v, want one folded source", m.Sources)
	}
	s := m.Sources[0]
	if s.Repo != "github.com/irasikhin/praktik-skills" || s.Ref != "v1" {
		t.Errorf("source repo/ref = %q/%q", s.Repo, s.Ref)
	}
	if !reflect.DeepEqual(s.Skills, []string{"proc", "lang-go"}) {
		t.Errorf("folded skills = %+v", s.Skills)
	}
}

func TestLoadManifestDupSource(t *testing.T) {
	data := "[[source]]\n  repo = \"a\"\n  skills = []\n[[source]]\n  repo = \"a\"\n  skills = []\n"
	path := filepath.Join(t.TempDir(), ManifestName)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected duplicate-source error")
	}
}

func TestLoadManifestAbsent(t *testing.T) {
	_, err := LoadManifest(filepath.Join(t.TempDir(), ManifestName))
	if !errors.Is(err, ErrNoManifest) {
		t.Fatalf("err = %v, want ErrNoManifest", err)
	}
}

func TestLoadManifestCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestName)
	if err := os.WriteFile(path, []byte("this = is = not = toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil || errors.Is(err, ErrNoManifest) {
		t.Fatalf("err = %v, want a parse error", err)
	}
}

func TestFindWalksAncestors(t *testing.T) {
	root := t.TempDir()
	if err := SaveManifest(filepath.Join(root, ManifestName), DefaultManifest()); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	p, ok, err := Find(nested)
	if err != nil || !ok {
		t.Fatalf("Find: ok=%v err=%v", ok, err)
	}
	// macOS /tmp symlinks through /private; compare resolved paths.
	if filepath.Base(p.Root()) != filepath.Base(root) {
		t.Errorf("root = %q, want %q", p.Root(), root)
	}
}

func TestFindNone(t *testing.T) {
	_, ok, err := Find(t.TempDir())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ok {
		t.Error("ok = true, want false in an empty dir")
	}
}

func TestProjectPaths(t *testing.T) {
	p := At("/tmp/proj")
	if got := p.ManifestPath(); !strings.HasSuffix(got, "/praktik.toml") {
		t.Errorf("ManifestPath = %q", got)
	}
	if got := p.LockPath(); !strings.HasSuffix(got, "/.praktik/lock.json") {
		t.Errorf("LockPath = %q", got)
	}
}

func TestWriteAtomicNoTempLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")
	if err := WriteAtomic(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "hello" {
		t.Fatalf("read = %q, %v", got, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "file.txt" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir entries = %v, want only file.txt (a temp file leaked)", names)
	}
}
