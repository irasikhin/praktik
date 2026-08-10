package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/project"
)

// TargetCmd manages the [targets] set in praktik.toml.
type TargetCmd struct {
	Add TargetAddCmd `cmd:"" help:"Add an agent target dir and vendor into it."`
	Rm  TargetRmCmd  `cmd:"" help:"Remove an agent target and prune its vendored copies."`
	Ls  TargetLsCmd  `cmd:"" help:"List configured targets."`
}

// TargetAddCmd adds an agent target and syncs into it.
type TargetAddCmd struct {
	Agent string `arg:"" help:"Agent name, e.g. claude, agents, codex, github, opencode, pi."`
	Path  string `arg:"" help:"Relative skills dir, e.g. .claude/skills."`
}

func (cmd *TargetAddCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, _, err := loadState(p)
	if err != nil {
		return err
	}
	if _, ok := m.Targets[cmd.Agent]; ok {
		return &UserError{Msg: fmt.Sprintf("target %q already exists", cmd.Agent)}
	}
	if m.Targets == nil {
		m.Targets = map[string]string{}
	}
	m.Targets[cmd.Agent] = cmd.Path
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return runSync(c, p, m)
}

// TargetRmCmd removes an agent target and prunes its vendored copies.
type TargetRmCmd struct {
	Agent string `arg:"" help:"Agent name to remove."`
}

func (cmd *TargetRmCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	m, lock, err := loadState(p)
	if err != nil {
		return err
	}
	rel, ok := m.Targets[cmd.Agent]
	if !ok {
		return &UserError{Msg: fmt.Sprintf("no such target %q", cmd.Agent)}
	}
	if len(m.Targets) == 1 {
		return &UserError{Msg: "cannot remove the last target"}
	}
	// Prune the dropped target's managed copies before removing it: the lock is
	// keyed by skill name only (not per-target), so the reconcile engine — which
	// iterates the remaining targets — would never clean these up.
	for _, name := range lock.Managed() {
		dest := filepath.Join(p.Root(), rel, name, "SKILL.md")
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return &UserError{Msg: err.Error()}
		}
		_ = os.Remove(filepath.Dir(dest))
	}
	delete(m.Targets, cmd.Agent)
	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	return runSync(c, p, m)
}

// TargetLsCmd lists configured targets.
type TargetLsCmd struct{}

func (cmd *TargetLsCmd) Run(c *ctx) error {
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
		return enc.Encode(m.Targets)
	}
	agents := make([]string, 0, len(m.Targets))
	for a := range m.Targets {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	for _, a := range agents {
		fmt.Fprintf(c.out, "%s  %s\n", color.Bold(a), color.Faint(m.Targets[a]))
	}
	return nil
}
