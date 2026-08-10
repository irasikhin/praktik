package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSkillsListSortsAndWraps(t *testing.T) {
	names := []string{
		"proc-zeta-rule", "arch-alpha-thing", "lang-go-style", "lang-go-testing",
		"build-maven-commands", "build-maven-quality", "tool-secrets-no-eyes",
		"proc-conventional-commits", "proc-repo-hygiene", "flow-root-cause",
	}
	got := renderSkillsList(names)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 80 {
			t.Errorf("line over 80 cols: %q", line)
		}
	}
	if !strings.HasPrefix(got, "arch-alpha-thing,") {
		t.Errorf("not sorted: %q", got)
	}
	for _, n := range names {
		if !strings.Contains(got, n) {
			t.Errorf("missing %q in %q", n, got)
		}
	}
	if strings.HasSuffix(got, "\n") || strings.HasSuffix(got, ",") {
		t.Errorf("trailing newline/comma: %q", got)
	}
}

func TestRenderSkillsListEmpty(t *testing.T) {
	if got := renderSkillsList(nil); got != "(none)" {
		t.Errorf("got %q, want (none)", got)
	}
}

func TestReplaceSkillsBlock(t *testing.T) {
	doc := "# Title\n\n<!-- praktik:skills BEGIN -->\nold, stale, list\n<!-- praktik:skills END -->\n\ntail\n"
	got, found, err := replaceSkillsBlock(doc, []string{"b-skill", "a-skill"})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	want := "# Title\n\n<!-- praktik:skills BEGIN -->\na-skill, b-skill\n<!-- praktik:skills END -->\n\ntail\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// Idempotent: replacing again changes nothing.
	again, _, _ := replaceSkillsBlock(got, []string{"b-skill", "a-skill"})
	if again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
}

func TestReplaceSkillsBlockNoMarkers(t *testing.T) {
	if _, found, err := replaceSkillsBlock("# plain doc\n", []string{"x"}); found || err != nil {
		t.Errorf("found=%v err=%v, want false/nil", found, err)
	}
}

func TestReplaceSkillsBlockBrokenMarkers(t *testing.T) {
	for _, doc := range []string{
		"<!-- praktik:skills BEGIN -->\nno end\n",
		"no begin\n<!-- praktik:skills END -->\n",
	} {
		if _, _, err := replaceSkillsBlock(doc, nil); err == nil {
			t.Errorf("no error for broken markers in %q", doc)
		}
	}
}

func TestSyncDocsAndStatus(t *testing.T) {
	root := t.TempDir()
	claude := filepath.Join(root, "CLAUDE.md")
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(claude,
		[]byte("# Doc\n<!-- praktik:skills BEGIN -->\n(none)\n<!-- praktik:skills END -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agents, []byte("# No markers here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	names := []string{"proc-x", "lang-go-y"}
	managed, stale, err := DocsStatus(root, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) != 1 || managed[0] != "CLAUDE.md" || len(stale) != 1 {
		t.Errorf("managed=%v stale=%v", managed, stale)
	}

	updated, err := SyncDocs(root, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0] != "CLAUDE.md" {
		t.Errorf("updated=%v", updated)
	}
	content, _ := os.ReadFile(claude)
	if !strings.Contains(string(content), "lang-go-y, proc-x") {
		t.Errorf("block not rewritten:\n%s", content)
	}
	// AGENTS.md untouched.
	if got, _ := os.ReadFile(agents); string(got) != "# No markers here\n" {
		t.Errorf("AGENTS.md touched: %q", got)
	}

	// Now clean: no drift, second sync is a no-op.
	if _, stale, _ := DocsStatus(root, names); len(stale) != 0 {
		t.Errorf("still stale: %v", stale)
	}
	if updated, _ := SyncDocs(root, names); len(updated) != 0 {
		t.Errorf("second sync rewrote: %v", updated)
	}
}

func TestReplaceSkillsBlockUpgradesLegacyMarkers(t *testing.T) {
	doc := "# Title\n\n<!-- praxis:skills BEGIN -->\nold, list\n<!-- praxis:skills END -->\n\ntail\n"
	got, found, err := replaceSkillsBlock(doc, []string{"b-skill", "a-skill"})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	want := "# Title\n\n<!-- praktik:skills BEGIN -->\na-skill, b-skill\n<!-- praktik:skills END -->\n\ntail\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// The upgraded doc is stable under a second replace.
	again, _, _ := replaceSkillsBlock(got, []string{"b-skill", "a-skill"})
	if again != got {
		t.Errorf("not idempotent after upgrade:\n%s", again)
	}
}

func TestReplaceSkillsBlockBrokenLegacyMarkers(t *testing.T) {
	for _, doc := range []string{
		"<!-- praxis:skills BEGIN -->\nno end\n",
		"no begin\n<!-- praxis:skills END -->\n",
	} {
		if _, _, err := replaceSkillsBlock(doc, nil); err == nil {
			t.Errorf("no error for broken legacy markers in %q", doc)
		}
	}
}
