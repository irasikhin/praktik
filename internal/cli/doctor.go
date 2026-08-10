package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/project"
	"github.com/irasikhin/praktik/internal/sync"
)

// DoctorCmd runs sanity checks on a project: the source resolves, every target
// is writable, the lock is in sync, any praktik:skills doc blocks match the
// lock, and there are no skillshare leftovers.
type DoctorCmd struct{}

func (cmd *DoctorCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, lock, err := loadState(p)
	if err != nil {
		return err
	}

	healthy := true
	pass := func(msg string) { fmt.Fprintln(c.out, color.Green("✓")+" "+msg) }
	warn := func(msg string) { fmt.Fprintln(c.out, color.Yellow("!")+" "+msg) }
	fail := func(msg string) { fmt.Fprintln(c.out, color.Red("✗")+" "+msg); healthy = false }

	sources, missing, serr := resolveSources(c, m)
	if serr != nil {
		fail("source does not resolve: " + serr.Error())
	} else {
		for _, s := range sources {
			pass(fmt.Sprintf("source %s resolves: %d skills @ %s", s.Key, len(s.Skills), shortCommit(s.Commit)))
		}
		for _, mm := range missing {
			warn(fmt.Sprintf("source %s: selector %q matched no skills", mm.Source, mm.Name))
		}
	}

	for _, agent := range sortedTargetNames(m.Targets) {
		dir := filepath.Join(p.Root(), m.Targets[agent])
		if writableDir(dir) {
			pass(fmt.Sprintf("target %q writable: %s", agent, dir))
		} else {
			fail(fmt.Sprintf("target %q not writable: %s", agent, dir))
		}
	}

	if serr == nil {
		req := sync.Request{Sources: sources, Targets: m.Targets, Root: p.Root(), Lock: lock, Missing: missing}
		if plan, perr := sync.Compute(req); perr != nil {
			fail("cannot compute plan: " + perr.Error())
		} else if plan.Changes() {
			warn(fmt.Sprintf("lock out of sync: +%d ~%d -%d (run 'praktik sync')",
				plan.Count(sync.Install), plan.Count(sync.Update), plan.Count(sync.Prune)))
		} else {
			pass("lock in sync with the manifest")
		}
	}

	managed, stale, derr := project.DocsStatus(p.Root(), lock.Managed())
	if derr != nil {
		fail("docs block: " + derr.Error())
	} else {
		staleSet := map[string]bool{}
		for _, d := range stale {
			staleSet[d] = true
		}
		for _, d := range managed {
			if staleSet[d] {
				warn(fmt.Sprintf("%s praktik:skills block stale (run 'praktik sync')", d))
			} else {
				pass(fmt.Sprintf("%s praktik:skills block in sync", d))
			}
		}
	}

	if p.Legacy() {
		warn("legacy praxis-named state (" + p.ManifestPath() + ") — run 'praktik migrate' to rename it")
	}
	if _, err := os.Stat(filepath.Join(p.Root(), ".skillshare")); err == nil {
		warn("leftover .skillshare/ found — run 'praktik migrate' to import it")
	}
	for _, agent := range sortedTargetNames(m.Targets) {
		leftover := filepath.Join(p.Root(), m.Targets[agent], ".skillshare-manifest.json")
		if _, err := os.Stat(leftover); err == nil {
			warn(fmt.Sprintf("leftover skillshare manifest: %s — remove it", leftover))
		}
	}

	if !healthy {
		return &UserError{Msg: ""}
	}
	return nil
}

func shortCommit(commit string) string {
	if len(commit) >= 7 && commit != "local" {
		return commit[:7]
	}
	if commit == "" {
		return "?"
	}
	return commit
}

func sortedTargetNames(targets map[string]string) []string {
	names := make([]string, 0, len(targets))
	for a := range targets {
		names = append(names, a)
	}
	sort.Strings(names)
	return names
}

// writableDir reports whether dir (or, if absent, its nearest existing ancestor)
// accepts a new file. It cleans up its probe file.
func writableDir(dir string) bool {
	d := nearestExistingDir(dir)
	f, err := os.CreateTemp(d, ".praktik-doctor-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func nearestExistingDir(dir string) string {
	for {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
