package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/project"
)

// SourceCmd manages the [[source]] set in praktik.toml.
type SourceCmd struct {
	Add SourceAddCmd `cmd:"" help:"Add a source repo (and optional selectors) and sync."`
	Rm  SourceRmCmd  `cmd:"" help:"Remove a source by alias or repo and prune its skills."`
	Ls  SourceLsCmd  `cmd:"" help:"List configured sources."`
}

// SourceAddCmd appends a source dependency and syncs.
type SourceAddCmd struct {
	Repo   string   `arg:"" help:"Git remote (github.com/o/r, a URL) or a local path."`
	Skills []string `arg:"" optional:"" help:"Selectors: axes/globs/names, or path:<repo-rel-dir>."`
	Root   string   `help:"Repo-relative subdir to scope discovery." placeholder:"DIR"`
	Name   string   `help:"Optional alias for this source." placeholder:"NAME"`
}

func (cmd *SourceAddCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	s := project.Source{Name: cmd.Name, Repo: cmd.Repo, Ref: c.ref, Root: cmd.Root, Skills: cmd.Skills}
	if s.Skills == nil {
		s.Skills = []string{}
	}
	for _, ex := range m.Sources {
		if ex.Key() == s.Key() {
			return &UserError{Msg: fmt.Sprintf("source %q already exists", s.Key())}
		}
	}
	// Replace a lone placeholder ambient source (from `init` with no --source).
	if len(m.Sources) == 1 && m.Sources[0].Repo == "" && len(m.Sources[0].Skills) == 0 {
		m.Sources[0] = s
	} else {
		m.Sources = append(m.Sources, s)
	}
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return runSync(c, p, m)
}

// SourceRmCmd removes a source and prunes its skills.
type SourceRmCmd struct {
	Key string `arg:"" help:"Source alias or repo to remove."`
}

func (cmd *SourceRmCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	kept := make([]project.Source, 0, len(m.Sources))
	found := false
	for _, s := range m.Sources {
		if s.Key() == cmd.Key || s.Repo == cmd.Key {
			found = true
			continue
		}
		kept = append(kept, s)
	}
	if !found {
		return &UserError{Msg: fmt.Sprintf("no source %q (see 'praktik source ls')", cmd.Key)}
	}
	m.Sources = kept
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return runSync(c, p, m)
}

// SourceLsCmd lists configured sources.
type SourceLsCmd struct{}

func (cmd *SourceLsCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	if c.json {
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(m.Sources)
	}
	for _, s := range m.Sources {
		repo := s.Repo
		if repo == "" {
			repo = "<ambient>"
		}
		if s.Ref != "" {
			repo += "@" + s.Ref
		}
		if s.Root != "" {
			repo += " (root " + s.Root + ")"
		}
		fmt.Fprintf(c.out, "%s  %s  %s\n", color.Bold(s.Key()), color.Faint(repo),
			color.Faint(fmt.Sprintf("[%d selector(s)]", len(s.Skills))))
	}
	return nil
}

// pickSource resolves which source a mutating command (add/rm) targets: the named
// one with --from, else the sole source (error if ambiguous).
func pickSource(m *project.Manifest, from string) (*project.Source, error) {
	if from != "" {
		for i := range m.Sources {
			if m.Sources[i].Key() == from || m.Sources[i].Repo == from {
				return &m.Sources[i], nil
			}
		}
		return nil, &UserError{Msg: fmt.Sprintf("no source %q (see 'praktik source ls')", from)}
	}
	if len(m.Sources) > 1 {
		return nil, &UserError{Msg: "multiple sources — pass --from <alias>"}
	}
	return m.PrimarySource(), nil
}
