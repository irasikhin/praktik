package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/handbook"
)

// SearchCmd matches a query (case-insensitive) against skill name, description,
// and axis.
type SearchCmd struct {
	Query string `arg:"" help:"Text matched against skill name, description, and axis."`
}

type searchItem struct {
	Name        string `json:"name"`
	Axis        string `json:"axis"`
	Description string `json:"description"`
}

func (cmd *SearchCmd) Run(c *ctx) error {
	lib, err := c.library()
	if err != nil {
		return err
	}
	skills, err := lib.Skills()
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	q := strings.ToLower(cmd.Query)
	var hits []searchItem
	for _, s := range skills {
		if strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.Description), q) ||
			strings.Contains(strings.ToLower(s.Axis), q) {
			hits = append(hits, searchItem{s.Name, s.Axis, s.Description})
		}
	}

	if c.json {
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(hits)
	}
	for _, h := range hits {
		fmt.Fprintf(c.out, "%s  %s\n", color.Bold(h.Name), color.Faint(handbook.Summarize(h.Description)))
	}
	fmt.Fprintln(c.out, color.Faint(fmt.Sprintf("%d match(es) for %q", len(hits), cmd.Query)))
	return nil
}
