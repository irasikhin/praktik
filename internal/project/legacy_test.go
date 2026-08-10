package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindLegacyManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, LegacyManifestName), []byte("skills = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, ok, err := Find(root)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !p.Legacy() {
		t.Error("legacy manifest not flagged as legacy")
	}
	if filepath.Base(p.ManifestPath()) != LegacyManifestName {
		t.Errorf("manifest path = %q", p.ManifestPath())
	}
	if !strings.Contains(p.LockPath(), ".praxis") {
		t.Errorf("lock path = %q, want under .praxis", p.LockPath())
	}
}

func TestFindPrefersNewManifestOverLegacy(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{ManifestName, LegacyManifestName} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("skills = []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, ok, err := Find(root)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if p.Legacy() || filepath.Base(p.ManifestPath()) != ManifestName {
		t.Errorf("legacy=%v path=%q, want the praktik manifest", p.Legacy(), p.ManifestPath())
	}
}

func TestMigrateProjectState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, LegacyManifestName), []byte("skills = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".praxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".praxis", "lock.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := MigrateProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 2 {
		t.Errorf("moved = %v, want 2 renames", moved)
	}
	if !fileExists(filepath.Join(root, ManifestName)) || !fileExists(filepath.Join(root, ".praktik", "lock.json")) {
		t.Error("new-name state missing after migrate")
	}
	if fileExists(filepath.Join(root, LegacyManifestName)) || fileExists(filepath.Join(root, ".praxis")) {
		t.Error("legacy state still present after migrate")
	}

	// Nothing left to move: second run is a no-op.
	if moved, err := MigrateProjectState(root); err != nil || len(moved) != 0 {
		t.Errorf("second run moved=%v err=%v", moved, err)
	}
}

func TestMigrateProjectStateConflict(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{ManifestName, LegacyManifestName} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("skills = []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := MigrateProjectState(root); err == nil {
		t.Error("no error when both manifests exist")
	}
}
