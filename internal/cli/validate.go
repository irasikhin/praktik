package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/validate"
)

// ValidateCmd lints every skill's frontmatter; a failure exits non-zero.
type ValidateCmd struct{}

func (cmd *ValidateCmd) Run(c *ctx) error {
	lib, err := c.library()
	if err != nil {
		return err
	}
	issues, n, err := validate.Run(lib)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if c.json {
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"checked": n,
			"ok":      len(issues) == 0,
			"issues":  issues,
		})
		if len(issues) > 0 {
			return &UserError{Msg: ""}
		}
		return nil
	}
	if len(issues) == 0 {
		fmt.Fprintln(c.out, color.Green(fmt.Sprintf("OK: %d skills valid", n)))
		return nil
	}
	fmt.Fprintln(c.errw, color.Red("SKILL.md validation FAILED:"))
	for _, is := range issues {
		fmt.Fprintf(c.errw, "  - %s: %s\n", is.Skill, is.Message)
	}
	return &UserError{Msg: ""}
}
