package sync

import (
	"fmt"
	"os"

	"github.com/irasikhin/praktik/internal/project"
)

// Apply executes the plan: install/update materialize the whole source skill
// directory into place via an atomic temp+rename swap; prune removes the managed
// directory tree. It only ever touches paths named in an Action, so foreign dirs
// in a target survive. The new lock is written only after every action succeeds,
// so an interrupted apply re-reconciles cleanly on the next run.
func Apply(req Request, p *Plan, lockPath string) error {
	desired, err := buildDesired(req.Sources)
	if err != nil {
		return err
	}
	for _, a := range p.Actions {
		switch a.Kind {
		case Install, Update:
			d, ok := desired[a.Skill]
			if !ok {
				return fmt.Errorf("internal: no source for skill %q", a.Skill)
			}
			if err := project.MaterializeDir(a.Path, d.skill.AbsDir, 0o644); err != nil {
				return fmt.Errorf("vendor %s: %w", a.Path, err)
			}
		case Prune:
			if err := os.RemoveAll(a.Path); err != nil {
				return fmt.Errorf("prune %s: %w", a.Path, err)
			}
		case Noop:
		}
	}
	return project.SaveLock(lockPath, p.NextLock)
}
