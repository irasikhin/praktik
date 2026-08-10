package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/handbook"
)

// ListCmd prints the available skills grouped by axis.
type ListCmd struct {
	Axis string `help:"Only list skills in this axis or group (e.g. lang, proc, lang-java)." placeholder:"AXIS"`
}

type listItem struct {
	Name        string `json:"name"`
	Axis        string `json:"axis"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

func (cmd *ListCmd) Run(c *ctx) error {
	lib, err := c.library()
	if err != nil {
		return err
	}
	skills, err := lib.Skills()
	if err != nil {
		return &UserError{Msg: err.Error()}
	}

	match := func(group, axis string) bool {
		return cmd.Axis == "" || cmd.Axis == group || cmd.Axis == axis
	}

	if c.json {
		items := make([]listItem, 0, len(skills))
		for _, s := range skills {
			g := handbook.Group(s.Axis)
			if match(g, s.Axis) {
				items = append(items, listItem{s.Name, s.Axis, g, s.Description})
			}
		}
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	count := 0
	for _, group := range handbook.Order {
		var lines []string
		for _, s := range skills {
			if handbook.Group(s.Axis) != group || !match(group, s.Axis) {
				continue
			}
			lines = append(lines, fmt.Sprintf("  %s  %s", color.Bold(s.Name), color.Faint(handbook.Summarize(s.Description))))
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintln(c.out, color.Cyan(handbook.Label[group]))
		for _, ln := range lines {
			fmt.Fprintln(c.out, ln)
			count++
		}
		fmt.Fprintln(c.out)
	}
	fmt.Fprintln(c.out, color.Faint(fmt.Sprintf("%d skills in %s", count, lib.Dir())))
	return nil
}
