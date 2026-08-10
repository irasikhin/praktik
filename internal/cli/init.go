package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/project"
)

// InitCmd scaffolds praktik.toml in the cwd and vendors the named skills.
type InitCmd struct {
	Skills []string `arg:"" optional:"" help:"Axes, globs, or names to seed (e.g. lang-java proc)."`
}

func (cmd *InitCmd) Run(c *ctx) error {
	p, err := initScope(c)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p.ManifestPath()); err == nil {
		return &UserError{Msg: fmt.Sprintf("%s already exists", p.ManifestPath())}
	}
	if legacy := filepath.Join(p.Root(), project.LegacyManifestName); !c.global {
		if _, err := os.Stat(legacy); err == nil {
			return &UserError{Msg: legacy + " exists — run 'praktik migrate' instead of init"}
		}
	}
	m := project.DefaultManifest()
	if len(cmd.Skills) > 0 {
		m.PrimarySource().Skills = cmd.Skills
	}
	// Persist an explicit --source/--ref into the manifest's first source so the
	// project pins it (e.g. a git remote at a tag) for everyone who clones it.
	if c.source != "" {
		m.PrimarySource().Repo = c.source
	}
	if c.ref != "" {
		m.PrimarySource().Ref = c.ref
	}
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	if !c.json {
		fmt.Fprintln(c.out, color.Green("created "+p.ManifestPath()))
	}
	return runSync(c, p, m)
}

// initScope picks where init writes: the global config in global mode, else a
// project rooted at the cwd.
func initScope(c *ctx) (*project.Project, error) {
	if c.global {
		p, err := project.Global()
		if err != nil {
			return nil, &UserError{Msg: err.Error()}
		}
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, &UserError{Msg: err.Error()}
	}
	return project.At(cwd), nil
}
