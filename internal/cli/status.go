package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/project"
	"github.com/irasikhin/praktik/internal/sync"
)

// StatusCmd shows drift between the manifest, the lock, the targets, and any
// praktik:skills doc blocks: +added ~changed -pruned, or =ok when everything
// matches.
type StatusCmd struct{}

type statusItem struct {
	Skill  string `json:"skill"`
	Source string `json:"source"`
	Target string `json:"target"`
	State  string `json:"state"` // added | changed | pruned | ok
}

type statusJSON struct {
	Items     []statusItem `json:"items"`
	Clean     bool         `json:"clean"`
	Missing   []string     `json:"missing"`
	DocsStale []string     `json:"docs_stale"`
}

func (cmd *StatusCmd) Run(c *ctx) error {
	p, err := c.project()
	if err != nil {
		return err
	}
	_, plan, err := planFor(c, p)
	if err != nil {
		return err
	}
	_, docsStale, err := project.DocsStatus(p.Root(), plan.NextLock.Managed())
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	if c.json {
		items := make([]statusItem, 0, len(plan.Actions))
		for _, a := range plan.Actions {
			items = append(items, statusItem{a.Skill, a.Source, a.Target, stateName(a.Kind)})
		}
		missing := make([]string, 0, len(plan.Missing))
		for _, m := range plan.Missing {
			missing = append(missing, m.Name)
		}
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(statusJSON{Items: items, Clean: !plan.Changes() && len(docsStale) == 0,
			Missing: missing, DocsStale: append([]string{}, docsStale...)})
	}

	for _, m := range plan.Missing {
		fmt.Fprintln(c.errw, color.Yellow(fmt.Sprintf("warning: %q (source %s) matched no skills", m.Name, m.Source)))
	}
	shown := 0
	for _, a := range plan.Actions {
		if a.Kind == sync.Noop {
			continue
		}
		where := a.Target
		if a.Source != "" {
			where = a.Source + " → " + a.Target
		}
		fmt.Fprintf(c.out, "%s %s %s\n", glyph(a.Kind), color.Bold(a.Skill), color.Faint(where))
		shown++
	}
	for _, d := range docsStale {
		fmt.Fprintf(c.out, "%s %s %s\n", color.Yellow("~"), color.Bold(d),
			color.Faint("praktik:skills block stale (run 'praktik sync')"))
		shown++
	}
	if shown == 0 {
		fmt.Fprintln(c.out, color.Faint("=ok — every target matches the manifest"))
	}
	return nil
}

func glyph(k sync.ActionKind) string {
	switch k {
	case sync.Install:
		return color.Green("+")
	case sync.Update:
		return color.Yellow("~")
	case sync.Prune:
		return color.Red("-")
	default:
		return color.Faint("=")
	}
}

func stateName(k sync.ActionKind) string {
	switch k {
	case sync.Install:
		return "added"
	case sync.Update:
		return "changed"
	case sync.Prune:
		return "pruned"
	default:
		return "ok"
	}
}
