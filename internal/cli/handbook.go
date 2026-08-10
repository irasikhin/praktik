package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/handbook"
)

// HandbookCmd regenerates HANDBOOK.md, or with --check fails when it is stale.
type HandbookCmd struct {
	Check bool `help:"Exit non-zero if HANDBOOK.md is stale (CI gate)."`
}

func (cmd *HandbookCmd) Run(c *ctx) error {
	lib, err := c.library()
	if err != nil {
		return err
	}
	content, n, err := handbook.Generate(lib)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	path := lib.HandbookPath()
	base := filepath.Base(path)
	if cmd.Check {
		cur, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return &UserError{Msg: err.Error()}
		}
		if string(cur) != content {
			return &UserError{Msg: fmt.Sprintf("%s is stale — run: praktik handbook", base)}
		}
		fmt.Fprintln(c.out, color.Green(fmt.Sprintf("OK: %s in sync", base)))
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return &UserError{Msg: err.Error()}
	}
	fmt.Fprintf(c.out, "wrote %s (%d skills)\n", base, n)
	return nil
}
