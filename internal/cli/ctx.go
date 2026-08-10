package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/irasikhin/praktik/internal/color"
	"github.com/irasikhin/praktik/internal/library"
	"github.com/irasikhin/praktik/internal/project"
)

// ctx is the runtime context bound into every command's Run method: stdio, the
// resolved global flags, and lazily-resolved skill library + project handles.
type ctx struct {
	in   io.Reader
	out  io.Writer
	errw io.Writer

	source string // --source override
	ref    string // --ref (git remote source)
	json   bool   // --json
	global bool   // -g/--global

	// refresh forces a fetch of every cached git source before resolving. Only
	// `praktik update` sets it; every other command uses the cache as-is.
	refresh bool

	lib  *library.Library
	proj *project.Project
}

// library resolves the skill library once, on first use.
func (c *ctx) library() (*library.Library, error) {
	if c.lib != nil {
		return c.lib, nil
	}
	lib, err := library.ResolveSource(c.source, c.ref)
	if err != nil {
		return nil, &UserError{Msg: err.Error()}
	}
	c.lib = lib
	return lib, nil
}

// project resolves the enclosing project once, on first use, by walking up from
// the cwd for a praktik.toml (the kpass openDatabase() lazy-open pattern). The
// consumption commands fail clearly when there is no project yet.
func (c *ctx) project() (*project.Project, error) {
	if c.proj != nil {
		return c.proj, nil
	}
	if c.global {
		p, err := project.Global()
		if err != nil {
			return nil, &UserError{Msg: err.Error()}
		}
		if _, statErr := os.Stat(p.ManifestPath()); statErr != nil {
			return nil, &UserError{Msg: "no global config found — run 'praktik -g init' first"}
		}
		c.warnLegacy(p)
		c.proj = p
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, &UserError{Msg: err.Error()}
	}
	p, ok, err := project.Find(cwd)
	if err != nil {
		return nil, &UserError{Msg: err.Error()}
	}
	if !ok {
		return nil, &UserError{Msg: "no praktik.toml found — run 'praktik init' first"}
	}
	c.warnLegacy(p)
	c.proj = p
	return p, nil
}

// warnLegacy nudges once toward `praktik migrate` when the resolved project
// still uses pre-rename praxis-named state.
func (c *ctx) warnLegacy(p *project.Project) {
	if p.Legacy() {
		fmt.Fprintln(c.errw, color.Yellow("legacy praxis-named state in use ("+p.ManifestPath()+") — run 'praktik migrate' to rename it"))
	}
}
