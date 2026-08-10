package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const markersOnly = "# Rules\n<!-- praktik:skills BEGIN -->\n<!-- praktik:skills END -->\n"

func TestSyncManagesSkillsBlock(t *testing.T) {
	lib, proj := initProject(t)
	claude := filepath.Join(proj, "CLAUDE.md")
	mustWrite(t, claude, markersOnly)

	// init fills the block.
	if code, _, errw := runProj(t, lib, "init", "proc-x"); code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}
	content, _ := os.ReadFile(claude)
	if !strings.Contains(string(content), "BEGIN -->\nproc-x\n<!--") {
		t.Fatalf("block not filled by init:\n%s", content)
	}

	// add extends it.
	if code, out, errw := runProj(t, lib, "add", "lang-go-y"); code != 0 {
		t.Fatalf("add exit=%d err=%s", code, errw)
	} else if !strings.Contains(out, "CLAUDE.md") {
		t.Errorf("add did not report the docs rewrite: %q", out)
	}
	content, _ = os.ReadFile(claude)
	if !strings.Contains(string(content), "lang-go-y, proc-x") {
		t.Fatalf("block not extended by add:\n%s", content)
	}

	// Hand-mangled block: status flags it, stays exit 0, then sync repairs it.
	mustWrite(t, claude, "# Rules\n<!-- praktik:skills BEGIN -->\nwrong\n<!-- praktik:skills END -->\n")
	code, out, _ := runProj(t, lib, "status")
	if code != 0 {
		t.Fatalf("status exit=%d", code)
	}
	if !strings.Contains(out, "praktik:skills block stale") {
		t.Errorf("status did not flag the stale block: %q", out)
	}
	if code, _, errw := runProj(t, lib, "sync"); code != 0 {
		t.Fatalf("sync exit=%d err=%s", code, errw)
	}
	content, _ = os.ReadFile(claude)
	if !strings.Contains(string(content), "lang-go-y, proc-x") {
		t.Fatalf("sync did not repair the block:\n%s", content)
	}

	// Clean again: status =ok, sync =ok.
	if _, out, _ := runProj(t, lib, "status"); !strings.Contains(out, "=ok") {
		t.Errorf("status not clean: %q", out)
	}
	if _, out, _ := runProj(t, lib, "sync"); !strings.Contains(out, "=ok") {
		t.Errorf("sync not =ok: %q", out)
	}
}

func TestStatusJSONReportsDocsStale(t *testing.T) {
	lib, proj := initProject(t)
	if code, _, errw := runProj(t, lib, "init", "proc-x"); code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}
	mustWrite(t, filepath.Join(proj, "AGENTS.md"), markersOnly)

	_, out, _ := runProj(t, lib, "--json", "status")
	if !strings.Contains(out, `"docs_stale": [`) || !strings.Contains(out, `"AGENTS.md"`) ||
		!strings.Contains(out, `"clean": false`) {
		t.Errorf("json status missing docs_stale: %s", out)
	}
}

func TestDoctorFlagsBrokenAndStaleBlocks(t *testing.T) {
	lib, proj := initProject(t)
	if code, _, errw := runProj(t, lib, "init", "proc-x"); code != 0 {
		t.Fatalf("init exit=%d err=%s", code, errw)
	}

	// Stale block → warn, still healthy.
	mustWrite(t, filepath.Join(proj, "CLAUDE.md"),
		"<!-- praktik:skills BEGIN -->\nwrong\n<!-- praktik:skills END -->\n")
	code, out, _ := runProj(t, lib, "doctor")
	if code != 0 {
		t.Fatalf("doctor exit=%d for stale block, want 0", code)
	}
	if !strings.Contains(out, "praktik:skills block stale") {
		t.Errorf("doctor did not warn about the stale block: %q", out)
	}

	// Lone BEGIN marker → hard fail.
	mustWrite(t, filepath.Join(proj, "CLAUDE.md"), "<!-- praktik:skills BEGIN -->\nno end\n")
	code, out, _ = runProj(t, lib, "doctor")
	if code == 0 {
		t.Fatal("doctor exit=0 for broken markers, want failure")
	}
	if !strings.Contains(out, "docs block") {
		t.Errorf("doctor did not report the broken markers: %q", out)
	}
}
