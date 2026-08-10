package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate(t *testing.T) {
	lib, proj := initProject(t)

	// Fake skillshare layout: config.yaml with dead temp sources, copied skills
	// under .skillshare/skills, and .claude/skills/<name> symlinks into them.
	cfg := `targets:
  - claude
skills:
  - name: proc-x
    source: /tmp/dead/proc-x
  - name: lang-go-y
    source: /tmp/dead/lang-go-y
`
	mustWrite(t, filepath.Join(proj, ".skillshare", "config.yaml"), cfg)
	mustWrite(t, filepath.Join(proj, ".claude", "skills", ".skillshare-manifest.json"), "{}\n")
	mustWrite(t, filepath.Join(proj, "CLAUDE.md"), markersOnly)
	for _, name := range []string{"proc-x", "lang-go-y"} {
		ssSkill := filepath.Join(proj, ".skillshare", "skills", name)
		mustWrite(t, filepath.Join(ssSkill, "SKILL.md"), "OLD COPY of "+name+"\n")
		link := filepath.Join(proj, ".claude", "skills", name)
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(ssSkill, link); err != nil {
			t.Fatal(err)
		}
	}

	if code, _, errw := runProj(t, lib, "migrate"); code != 0 {
		t.Fatalf("migrate exit=%d err=%s", code, errw)
	}

	// praktik.toml written with the imported skills.
	if !fileExists(filepath.Join(proj, "praktik.toml")) {
		t.Fatal("praktik.toml not written")
	}
	// The skillshare dir is gone.
	if fileExists(filepath.Join(proj, ".skillshare")) {
		t.Error(".skillshare/ not removed")
	}
	// The target now holds a REAL dir + file (not a symlink), with source content.
	skillDir := filepath.Join(proj, ".claude", "skills", "proc-x")
	fi, err := os.Lstat(skillDir)
	if err != nil {
		t.Fatalf("migrated skill dir missing: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("migrated skill dir is still a symlink")
	}
	got, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	want, _ := os.ReadFile(filepath.Join(lib, "skills", "proc-x", "SKILL.md"))
	if string(got) != string(want) || strings.Contains(string(got), "OLD COPY") {
		t.Error("migrated copy is not the fresh source content")
	}
	// Skillshare's per-target manifest is gone too.
	if fileExists(filepath.Join(proj, ".claude", "skills", ".skillshare-manifest.json")) {
		t.Error(".skillshare-manifest.json not removed")
	}
	// The praktik:skills block got filled by the migration's sync.
	doc, _ := os.ReadFile(filepath.Join(proj, "CLAUDE.md"))
	if !strings.Contains(string(doc), "lang-go-y, proc-x") {
		t.Errorf("praktik:skills block not filled:\n%s", doc)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
