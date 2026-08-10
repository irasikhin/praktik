package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/skill"
)

// NewCmd scaffolds a new skills/<name>/SKILL.md in the library with a frontmatter
// template. Point --source at your local praktik checkout to author there.
type NewCmd struct {
	Name        string `arg:"" help:"New skill name — a kebab-case dir, e.g. lang-rust-style."`
	Axis        string `help:"metadata.axis (default: inferred from the name prefix)." placeholder:"AXIS"`
	Description string `short:"d" help:"One-line description (10–1024 chars)." placeholder:"TEXT"`
}

func (cmd *NewCmd) Run(c *ctx) error {
	if !skill.IsKebab(cmd.Name) {
		return &UserError{Msg: fmt.Sprintf("skill name %q must be kebab-case (a-z0-9, hyphen-separated)", cmd.Name)}
	}
	lib, err := c.library()
	if err != nil {
		return err
	}
	dir := filepath.Join(lib.Dir(), cmd.Name)
	path := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(path); err == nil {
		return &UserError{Msg: fmt.Sprintf("skill %q already exists at %s", cmd.Name, path)}
	}

	axis := cmd.Axis
	if axis == "" {
		axis = inferAxis(cmd.Name)
	}
	desc := cmd.Description
	if desc == "" {
		desc = "TODO: describe when an agent should use this skill (10–1024 chars)."
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\nmetadata:\n  axis: %s\n---\n\n# %s\n\nTODO: write the skill body.\n",
		cmd.Name, desc, axis, cmd.Name)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &UserError{Msg: err.Error()}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &UserError{Msg: err.Error()}
	}
	if !c.json {
		fmt.Fprintln(c.out, color.Green("created "+path))
	}
	return nil
}

// inferAxis guesses metadata.axis from the name: lang-*/build-* skills carry a
// two-segment axis (lang-java), others a single-segment one (proc).
func inferAxis(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) >= 2 && (parts[0] == "lang" || parts[0] == "build") {
		return parts[0] + "-" + parts[1]
	}
	return parts[0]
}
