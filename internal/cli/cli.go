// Package cli implements the praktik command-line interface. Argument parsing and
// help rendering are delegated to alecthomas/kong; this file orchestrates kong
// parse + dispatch and reshapes errors into the UserError stderr format.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/irasikhin/praktik/internal/color"
)

// Run is the entry point invoked by cmd/praktik/main.go.
func Run(argv []string, in io.Reader, out, errw io.Writer) int {
	color.Init()
	c := &ctx{in: in, out: out, errw: errw}
	if err := runOnce(c, argv); err != nil {
		var ue *UserError
		if errors.As(err, &ue) {
			if ue.Msg != "" {
				fmt.Fprintln(errw, color.Red(ue.Msg))
			}
			return 1
		}
		fmt.Fprintln(errw, color.Red(err.Error()))
		return 1
	}
	return 0
}

func runOnce(c *ctx, argv []string) error {
	cli := &praktikCLI{}
	app, err := newKongApp(cli, c)
	if err != nil {
		return &UserError{Msg: err.Error()}
	}
	if len(argv) == 0 {
		argv = []string{"--help"}
	}

	// Recover from kong's exits (help, version, error printing) so they
	// short-circuit cleanly without os.Exit.
	var kctx *kong.Context
	var exit *kongExit
	func() {
		defer func() {
			if r := recover(); r != nil {
				if ke, ok := r.(kongExit); ok {
					exit = &ke
					return
				}
				panic(r)
			}
		}()
		kctx, err = app.Parse(argv)
	}()
	if exit != nil {
		if exit.code != 0 {
			return &UserError{Msg: ""}
		}
		return nil
	}
	if err != nil {
		return &UserError{Msg: reshapeKongError(err)}
	}

	c.source = cli.Source
	c.ref = cli.Ref
	c.json = cli.JSON
	c.global = cli.Global
	if cli.NoColor {
		color.Disable()
	}
	return kctx.Run(c)
}

// kongExit is the panic value used to intercept kong.Exit invocations.
type kongExit struct{ code int }

func newKongApp(cli *praktikCLI, c *ctx) (*kong.Kong, error) {
	return kong.New(
		cli,
		kong.Name("praktik"),
		kong.Description("Vendor and author praktik skills: init/add/sync into agent targets, plus list, show, validate, and the handbook."),
		kong.UsageOnError(),
		kong.Writers(c.out, c.errw),
		kong.Bind(c),
		kong.Vars{"version": versionLine()},
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		}),
		kong.Exit(func(code int) {
			panic(kongExit{code})
		}),
	)
}

func reshapeKongError(err error) string {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "praktik: error: ")
	msg = strings.TrimPrefix(msg, "praktik: ")
	return "praktik: " + msg + "\nUse 'praktik --help' for usage."
}
