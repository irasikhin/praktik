package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/skill"
	"github.com/irasikhin/praktik/internal/sync"
)

// DiffCmd shows the per-file diff between a vendored skill directory and its
// source for skills that have drifted — the detail behind status's ~changed. The
// diff is oriented vendored→source, so '+' lines are what a sync would add.
type DiffCmd struct {
	Name string `arg:"" optional:"" help:"Skill name; omit to diff every changed skill."`
}

type diffJSON struct {
	Skill  string         `json:"skill"`
	Source string         `json:"source"`
	Target string         `json:"target"`
	Files  []fileDiffJSON `json:"files"`
}

type fileDiffJSON struct {
	Path     string `json:"path"`
	Source   string `json:"source"`
	Vendored string `json:"vendored"`
}

func (cmd *DiffCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	req, plan, err := planFor(c, p)
	if err != nil {
		return err
	}

	var changed []sync.Action
	for _, a := range plan.Actions {
		if a.Kind != sync.Update {
			continue
		}
		if cmd.Name != "" && a.Skill != cmd.Name {
			continue
		}
		changed = append(changed, a)
	}

	if c.json {
		out := make([]diffJSON, 0, len(changed))
		for _, a := range changed {
			src := findChosen(req, a.Skill)
			if src == nil {
				return &UserError{Msg: fmt.Sprintf("no source for skill %q", a.Skill)}
			}
			dj := diffJSON{Skill: a.Skill, Source: a.Source, Target: a.Target}
			for _, rel := range unionFiles(src.AbsDir, a.Path) {
				sb, _ := os.ReadFile(filepath.Join(src.AbsDir, rel))
				vb, _ := os.ReadFile(filepath.Join(a.Path, rel))
				if string(sb) == string(vb) {
					continue
				}
				dj.Files = append(dj.Files, fileDiffJSON{Path: rel, Source: string(sb), Vendored: string(vb)})
			}
			out = append(out, dj)
		}
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(changed) == 0 {
		if cmd.Name != "" {
			fmt.Fprintln(c.out, color.Faint(fmt.Sprintf("%s is in sync", cmd.Name)))
		} else {
			fmt.Fprintln(c.out, color.Faint("nothing changed"))
		}
		return nil
	}

	for _, a := range changed {
		src := findChosen(req, a.Skill)
		if src == nil {
			return &UserError{Msg: fmt.Sprintf("no source for skill %q", a.Skill)}
		}
		fmt.Fprintln(c.out, color.Bold(fmt.Sprintf("%s (%s → %s)", a.Skill, a.Source, a.Target)))
		for _, rel := range unionFiles(src.AbsDir, a.Path) {
			sb, _ := os.ReadFile(filepath.Join(src.AbsDir, rel))
			vb, _ := os.ReadFile(filepath.Join(a.Path, rel))
			if string(sb) == string(vb) {
				continue
			}
			fmt.Fprintln(c.out, color.Faint("· "+rel))
			for _, line := range diffLines(splitLines(vb), splitLines(sb)) {
				switch line[0] {
				case '+':
					fmt.Fprintln(c.out, color.Green(line))
				case '-':
					fmt.Fprintln(c.out, color.Red(line))
				default:
					fmt.Fprintln(c.out, color.Faint(line))
				}
			}
		}
		fmt.Fprintln(c.out)
	}
	return nil
}

// findChosen returns the chosen source skill by name (carrying its AbsDir).
func findChosen(req sync.Request, name string) *skill.Skill {
	for _, s := range req.Sources {
		for _, sk := range s.Skills {
			if sk.Name == name {
				return sk
			}
		}
	}
	return nil
}

// unionFiles returns the sorted union of regular-file relative paths under a and b.
func unionFiles(a, b string) []string {
	set := map[string]bool{}
	for _, d := range []string{a, b} {
		_ = filepath.WalkDir(d, func(pth string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				if e.Name() == ".git" && pth != d {
					return fs.SkipDir
				}
				return nil
			}
			if e.Type().IsRegular() {
				rel, _ := filepath.Rel(d, pth)
				set[filepath.ToSlash(rel)] = true
			}
			return nil
		})
	}
	out := make([]string, 0, len(set))
	for rel := range set {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(string(b), "\n")
}
