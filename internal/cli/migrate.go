package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/project"
)

// MigrateCmd upgrades older on-disk state to the current praktik layout. It
// renames pre-rename praxis-named state (praxis.toml/.praxis, or the legacy
// global config/state files) and re-syncs so doc markers upgrade too; when no
// praxis state exists it imports a skillshare setup (.skillshare/config.yaml)
// into a praktik.toml + lock, vendors real copies, and removes the old
// .skillshare/ dir and the symlinks it left in the targets.
type MigrateCmd struct{}

// skillshareConfig is the subset of .skillshare/config.yaml we read.
type skillshareConfig struct {
	Targets []string `yaml:"targets"`
	Skills  []struct {
		Name string `yaml:"name"`
	} `yaml:"skills"`
}

func (cmd *MigrateCmd) Run(c *ctx) error {
	if done, err := migrateLegacyState(c); done || err != nil {
		return err
	}

	p, err := initScope(c)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p.ManifestPath()); err == nil {
		return &UserError{Msg: p.ManifestPath() + " already exists — nothing to migrate"}
	}

	ssDir := filepath.Join(p.Root(), ".skillshare")
	data, err := os.ReadFile(filepath.Join(ssDir, "config.yaml"))
	if err != nil {
		return &UserError{Msg: "no .skillshare/config.yaml to migrate: " + err.Error()}
	}
	var cfg skillshareConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &UserError{Msg: "parse .skillshare/config.yaml: " + err.Error()}
	}

	m := project.DefaultManifest()
	m.Targets = map[string]string{}
	for _, agent := range cfg.Targets {
		m.Targets[agent] = targetPath(agent)
	}
	if len(m.Targets) == 0 {
		m.Targets["claude"] = ".claude/skills"
	}
	names := make([]string, 0, len(cfg.Skills))
	for _, s := range cfg.Skills {
		if s.Name != "" {
			names = append(names, s.Name)
		}
	}
	sort.Strings(names)
	m.PrimarySource().Skills = names

	// Remove the old skillshare symlinks first, so sync writes real dirs instead
	// of following the symlink and writing back into .skillshare. The per-target
	// .skillshare-manifest.json is skillshare's, not ours — drop it too.
	for _, rel := range m.Targets {
		_ = os.Remove(filepath.Join(p.Root(), rel, ".skillshare-manifest.json"))
		for _, name := range names {
			link := filepath.Join(p.Root(), rel, name)
			if fi, lerr := os.Lstat(link); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
				_ = os.Remove(link)
			}
		}
	}

	if err := project.SaveManifest(p.ManifestPath(), m); err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := runSync(c, p, m); err != nil {
		return err
	}
	if err := os.RemoveAll(ssDir); err != nil {
		return &UserError{Msg: "could not remove .skillshare/: " + err.Error()}
	}
	if !c.json {
		fmt.Fprintln(c.out, color.Green(fmt.Sprintf("migrated %d skills; removed .skillshare/", len(names))))
	}
	return nil
}

// migrateLegacyState renames praxis-named state to the praktik names when any
// exists in the requested scope, then re-syncs (which also upgrades legacy
// praxis:skills doc markers). done is false when there was nothing to rename,
// letting Run fall through to the skillshare import.
func migrateLegacyState(c *ctx) (done bool, err error) {
	var moved []string
	var p *project.Project
	if c.global {
		if moved, err = project.MigrateGlobalState(); err != nil {
			return false, &UserError{Msg: err.Error()}
		}
		if len(moved) > 0 {
			if p, err = project.Global(); err != nil {
				return false, &UserError{Msg: err.Error()}
			}
		}
	} else {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return false, &UserError{Msg: cwdErr.Error()}
		}
		if lp, ok, findErr := project.Find(cwd); findErr != nil {
			return false, &UserError{Msg: findErr.Error()}
		} else if ok && lp.Legacy() {
			if moved, err = project.MigrateProjectState(lp.Root()); err != nil {
				return false, &UserError{Msg: err.Error()}
			}
			p = project.At(lp.Root())
		}
	}
	if len(moved) == 0 {
		return false, nil
	}

	m, err := project.LoadManifest(p.ManifestPath())
	if err != nil {
		return true, &UserError{Msg: err.Error()}
	}
	if err := runSync(c, p, m); err != nil {
		return true, err
	}
	if !c.json {
		for _, mv := range moved {
			fmt.Fprintln(c.out, color.Green("renamed "+mv))
		}
	}
	return true, nil
}

// targetPath maps a skillshare agent name to its conventional skills dir.
func targetPath(agent string) string {
	switch agent {
	case "claude":
		return ".claude/skills"
	case "codex":
		return ".codex/skills"
	case "opencode":
		return ".config/opencode/skills"
	case "pi":
		return ".pi/agent/skills"
	default:
		return filepath.Join("."+agent, "skills")
	}
}
