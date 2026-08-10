package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skills-block markers: an agent doc (CLAUDE.md / AGENTS.md) opts into a
// tool-managed list of vendored skills by carrying this marker pair on lines of
// its own; every reconcile rewrites the lines between them to the lock's
// managed set, so the doc can never drift from praktik.toml. Only the first
// marker pair in a file is managed.
const (
	skillsBlockBegin = "<!-- praktik:skills BEGIN -->"
	skillsBlockEnd   = "<!-- praktik:skills END -->"

	// Pre-rename marker pair, still recognized so docs written by the praxis
	// CLI stay managed; the next write upgrades them to the praktik markers.
	legacySkillsBlockBegin = "<!-- praxis:skills BEGIN -->"
	legacySkillsBlockEnd   = "<!-- praxis:skills END -->"
)

// DocsFiles are the agent-doc filenames (relative to the project root) scanned
// for a praktik:skills marker block.
var DocsFiles = []string{"CLAUDE.md", "AGENTS.md"}

// SyncDocs rewrites the praktik:skills block in every docs file that has one to
// the given skill names, returning the files it actually changed. Files
// without markers — or absent entirely — are left alone: the block is opt-in.
func SyncDocs(root string, names []string) (updated []string, err error) {
	_, stale, err := visitDocs(root, names, func(path string, content []byte) error {
		return WriteAtomic(path, content, 0o644)
	})
	return stale, err
}

// DocsStatus reports which docs files carry a praktik:skills block (managed)
// and which of those no longer match the given names (stale), without writing.
func DocsStatus(root string, names []string) (managed, stale []string, err error) {
	return visitDocs(root, names, nil)
}

// visitDocs scans DocsFiles under root for a marker block, collecting the
// files that have one and the subset whose block content is out of date; when
// write is non-nil each stale file's rewritten content is handed to it.
func visitDocs(root string, names []string, write func(path string, content []byte) error) (managed, stale []string, err error) {
	for _, name := range DocsFiles {
		path := filepath.Join(root, name)
		content, rerr := os.ReadFile(path)
		if os.IsNotExist(rerr) {
			continue
		}
		if rerr != nil {
			return nil, nil, rerr
		}
		updated, found, berr := replaceSkillsBlock(string(content), names)
		if berr != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, berr)
		}
		if !found {
			continue
		}
		managed = append(managed, name)
		if updated == string(content) {
			continue
		}
		stale = append(stale, name)
		if write != nil {
			if werr := write(path, []byte(updated)); werr != nil {
				return nil, nil, werr
			}
		}
	}
	return managed, stale, nil
}

// replaceSkillsBlock rewrites the first marker block in content to hold names.
// found is false when content has no block at all; a lone BEGIN or END marker
// is an error — a silently unmanaged block would just drift. A legacy
// praxis:skills pair is managed too and rewritten with the praktik markers.
func replaceSkillsBlock(content string, names []string) (updated string, found bool, err error) {
	for _, markers := range [][2]string{
		{skillsBlockBegin, skillsBlockEnd},
		{legacySkillsBlockBegin, legacySkillsBlockEnd},
	} {
		blockBegin, blockEnd := markers[0], markers[1]
		begin := strings.Index(content, blockBegin)
		if begin < 0 {
			if strings.Contains(content, blockEnd) {
				return "", false, fmt.Errorf("found %q without %q", blockEnd, blockBegin)
			}
			continue
		}
		rest := content[begin+len(blockBegin):]
		end := strings.Index(rest, blockEnd)
		if end < 0 {
			return "", false, fmt.Errorf("found %q without %q", blockBegin, blockEnd)
		}
		return content[:begin] + skillsBlockBegin + "\n" + renderSkillsList(names) + "\n" +
			skillsBlockEnd + rest[end+len(blockEnd):], true, nil
	}
	return "", false, nil
}

// renderSkillsList formats names as a sorted comma-separated list greedily
// wrapped at 80 columns — compact in the doc, deterministic for diffs.
func renderSkillsList(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	var b strings.Builder
	line := ""
	for i, n := range sorted {
		item := n
		if i < len(sorted)-1 {
			item += ","
		}
		switch {
		case line == "":
			line = item
		case len(line)+1+len(item) <= 80:
			line += " " + item
		default:
			b.WriteString(line + "\n")
			line = item
		}
	}
	b.WriteString(line)
	return b.String()
}
