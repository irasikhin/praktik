package cli

import (
	"encoding/json"
	"fmt"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/skill"
)

// ShowCmd renders one skill's frontmatter and body.
type ShowCmd struct {
	Name string `arg:"" help:"Skill name (its directory), e.g. lang-java-naming."`
}

type showJSON struct {
	Name        string `json:"name"`
	Axis        string `json:"axis"`
	Description string `json:"description"`
	Hash        string `json:"hash"`
	Body        string `json:"body"`
}

func (cmd *ShowCmd) Run(c *ctx) error {
	lib, err := c.library()
	if err != nil {
		return err
	}
	s, err := lib.Skill(cmd.Name)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if c.json {
		enc := json.NewEncoder(c.out)
		enc.SetIndent("", "  ")
		return enc.Encode(showJSON{s.Name, s.Axis, s.Description, s.Hash, s.Body})
	}
	printSkillText(c, s)
	return nil
}

// printSkillText renders a skill's header + body to stdout. Shared by show and
// the interactive picker.
func printSkillText(c *ctx, s *skill.Skill) {
	header := color.Bold(s.Name)
	if s.Axis != "" {
		header += "  " + color.Faint(s.Axis)
	}
	fmt.Fprintln(c.out, header)
	if s.Description != "" {
		fmt.Fprintln(c.out, s.Description)
	}
	fmt.Fprintln(c.out, color.Faint("────────"))
	fmt.Fprintln(c.out, s.Body)
}
