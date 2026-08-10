package cli

import (
	"fmt"

	"github.com/irasikhin/praktik/internal/color"
)

// UpdateCmd force-fetches every cached git source, re-resolves skills, and
// rewrites the lock to match — the one command that hits the network for an
// already-cached source (sync/status/diff/add/rm use the cache as-is). A local
// library source has no fetch step: it is simply re-read and any skill whose
// source bytes changed is re-copied (rewriting its lock hash). With patterns
// the reconcile is scoped to the matching skills; everything else is frozen
// (untouched on disk, lock entry preserved verbatim).
type UpdateCmd struct {
	Patterns []string `arg:"" optional:"" help:"Limit the update to skills matching these axes/globs/names."`
}

func (cmd *UpdateCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	if !c.json {
		fmt.Fprintln(c.out, color.Faint("re-resolving from sources (fetching git remotes)…"))
	}
	c.refresh = true
	return runSync(c, p, m, cmd.Patterns...)
}
