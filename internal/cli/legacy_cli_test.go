package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyMarkersOnly = "# Rules\n<!-- praxis:skills BEGIN -->\n<!-- praxis:skills END -->\n"

// legacyProject scaffolds a real project via init, then renames its state back
// to the pre-rename praxis names.
func legacyProject(t *testing.T) (lib, proj string) {
	t.Helper()
	lib, proj = initProject(t)
	if code, _, errw := runProj(t, lib, "init", "proc-x"); code != 0 {
		t.Fatalf("init exit != 0: %s", errw)
	}
	if err := os.Rename(filepath.Join(proj, "praktik.toml"), filepath.Join(proj, "praxis.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(proj, ".praktik"), filepath.Join(proj, ".praxis")); err != nil {
		t.Fatal(err)
	}
	return lib, proj
}

func TestLegacyManifestStillWorksAndWarns(t *testing.T) {
	lib, _ := legacyProject(t)
	code, _, errw := runProj(t, lib, "status")
	if code != 0 {
		t.Fatalf("status on legacy project exit=%d err=%s", code, errw)
	}
	if !strings.Contains(errw, "praktik migrate") {
		t.Errorf("no migrate hint on stderr: %q", errw)
	}
}

func TestMigrateRenamesLegacyState(t *testing.T) {
	lib, proj := legacyProject(t)
	mustWrite(t, filepath.Join(proj, "CLAUDE.md"), legacyMarkersOnly)

	code, _, errw := runProj(t, lib, "migrate")
	if code != 0 {
		t.Fatalf("migrate exit=%d err=%s", code, errw)
	}
	if !fileExists(filepath.Join(proj, "praktik.toml")) || !fileExists(filepath.Join(proj, ".praktik", "lock.json")) {
		t.Error("praktik-named state missing after migrate")
	}
	if fileExists(filepath.Join(proj, "praxis.toml")) || fileExists(filepath.Join(proj, ".praxis")) {
		t.Error("legacy praxis-named state left behind")
	}
	doc, _ := os.ReadFile(filepath.Join(proj, "CLAUDE.md"))
	if !strings.Contains(string(doc), "<!-- praktik:skills BEGIN -->") || !strings.Contains(string(doc), "proc-x") {
		t.Errorf("legacy markers not upgraded:\n%s", doc)
	}
	// Vendored skill still in place after the rename + re-sync.
	if !fileExists(vendored(proj, ".claude/skills", "proc-x")) {
		t.Error("vendored skill missing after migrate")
	}
}

func TestInitRefusesNextToLegacyManifest(t *testing.T) {
	lib, _ := legacyProject(t)
	code, _, errw := runProj(t, lib, "init")
	if code == 0 {
		t.Fatal("init succeeded next to praxis.toml")
	}
	if !strings.Contains(errw, "praktik migrate") {
		t.Errorf("no migrate hint: %q", errw)
	}
}
