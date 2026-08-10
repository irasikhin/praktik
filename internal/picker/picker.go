// Package picker provides an interactive selector over a list of lines using
// fzf, with a test injection seam. Ported near-verbatim from kpass.
package picker

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PickOpts tweaks the fzf invocation. The zero value disables all decorations.
type PickOpts struct {
	Preview   string // --preview expression
	Delimiter string // --delimiter for multi-column input
	WithNth   string // --with-nth visible columns
}

// Hook is the test injection seam: when set, Pick forwards to it instead of fzf.
var Hook func(lines []string, query string) (string, error)

// Seams for unit tests; defaults call into the real environment.
var (
	lookPathFn = exec.LookPath
	runCmdFn   = func(c *exec.Cmd) error { return c.Run() }
)

// Pick presents lines in fzf and returns the chosen line. With a single line and
// --select-1 it returns immediately; an empty selection is an error.
func Pick(lines []string, query string, opts PickOpts) (string, error) {
	if Hook != nil {
		return Hook(lines, query)
	}
	if len(lines) == 0 {
		return "", errors.New("no entries found")
	}

	binary, err := lookPathFn("fzf")
	if err != nil {
		return "", errors.New("fzf not found. Install: https://github.com/junegunn/fzf")
	}

	args := []string{"--select-1", "--exit-0"}
	if query != "" {
		args = append(args, "-q", query)
	}
	if opts.Preview != "" {
		args = append(args, "--preview", opts.Preview)
	}
	if opts.Delimiter != "" {
		args = append(args, "--delimiter", opts.Delimiter)
	}
	if opts.WithNth != "" {
		args = append(args, "--with-nth", opts.WithNth)
	}

	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := runCmdFn(cmd); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if code := exitErr.ExitCode(); code != 0 && code != 1 {
				return "", fmt.Errorf("fzf exited with status %d: %s", code, strings.TrimSpace(string(exitErr.Stderr)))
			}
		} else {
			return "", err
		}
	}
	selected := strings.TrimSpace(out.String())
	if selected == "" {
		return "", errors.New("no entry selected")
	}
	return selected, nil
}
