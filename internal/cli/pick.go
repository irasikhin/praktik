package cli

import (
	"fmt"
	"strings"

	"github.com/irasikhin/praktik/internal/handbook"
	"github.com/irasikhin/praktik/internal/picker"
)

// PickCmd runs an interactive fzf picker over the library. By default it shows
// the chosen skill; --add adds it to the manifest and syncs.
type PickCmd struct {
	Add bool `help:"Add the picked skill to the manifest and sync (default: show it)."`
}

func (cmd *PickCmd) Run(c *ctx) error {
	lib, err := c.library()
	if err != nil {
		return err
	}
	skills, err := lib.Skills()
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	lines := make([]string, 0, len(skills))
	for _, s := range skills {
		lines = append(lines, fmt.Sprintf("%s\t%s", s.Name, handbook.Summarize(s.Description)))
	}

	sel, err := picker.Pick(lines, "", picker.PickOpts{Delimiter: "\t", WithNth: "1,2"})
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	name := strings.TrimSpace(strings.SplitN(sel, "\t", 2)[0])
	if name == "" {
		return &UserError{Msg: "no skill selected"}
	}

	if cmd.Add {
		return addToManifest(c, []string{name}, "")
	}
	s, err := lib.Skill(name)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	printSkillText(c, s)
	return nil
}
