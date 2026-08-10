package cli

import (
	"sort"

	"github.com/irasikhin/praktik/internal/project"
)

// AddCmd adds skill selectors to a source's selection, then syncs.
type AddCmd struct {
	Patterns []string `arg:"" help:"Axes, globs, or names to add (e.g. build-maven lang-java)."`
	From     string   `help:"Alias/repo of the source to add to (default: the sole source)." placeholder:"SRC"`
}

func (cmd *AddCmd) Run(c *ctx) error {
	return addToManifest(c, cmd.Patterns, cmd.From)
}

// addToManifest unions patterns into the chosen source's skill selection and
// syncs. Shared by `add` and `pick --add`. Selectors that match nothing are
// reported by runSync's missing-warning (kept in the manifest).
func addToManifest(c *ctx, patterns []string, from string) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	src, err := pickSource(m, from)
	if err != nil {
		return err
	}
	src.Skills = unionSorted(src.Skills, patterns)
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return runSync(c, p, m)
}

func unionSorted(existing, add []string) []string {
	set := make(map[string]bool, len(existing)+len(add))
	for _, s := range existing {
		set[s] = true
	}
	for _, s := range add {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
