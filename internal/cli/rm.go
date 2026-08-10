package cli

import "github.com/irasikhin/praktik/internal/project"

// RmCmd removes skill patterns from the manifest, then syncs — the reconcile
// engine prunes any skill that is no longer desired. Patterns are matched as
// exact manifest tokens (e.g. removing "proc" drops the "proc" entry).
type RmCmd struct {
	Patterns []string `arg:"" help:"Axes, globs, or names to remove from the manifest."`
	From     string   `help:"Alias/repo of the source to remove from (default: the sole source)." placeholder:"SRC"`
}

func (cmd *RmCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	src, err := pickSource(m, cmd.From)
	if err != nil {
		return err
	}
	src.Skills = removeTokens(src.Skills, cmd.Patterns)
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return runSync(c, p, m)
}

func removeTokens(existing, drop []string) []string {
	rm := make(map[string]bool, len(drop))
	for _, s := range drop {
		rm[s] = true
	}
	out := make([]string, 0, len(existing))
	for _, s := range existing {
		if !rm[s] {
			out = append(out, s)
		}
	}
	return out
}
