// Package sync is the reconcile engine: given the skills selected from each
// source, the agent targets, and the current lock, it computes a Plan (per skill,
// per target: install / update / prune / noop) and applies it as atomic real
// directory copies — never symlinks — pruning only entries it previously recorded
// in the lock. Two sources providing the same skill name is a hard error (never a
// silent shadow). add/rm/sync/update/init/source all funnel through here.
package sync

import (
	"fmt"
	"path/filepath"

	"github.com/irasikhin/praktik/internal/project"
	"github.com/irasikhin/praktik/internal/skill"
)

// Request is everything the engine needs; the CLI assembles it by resolving and
// selecting each manifest source.
type Request struct {
	Sources []ResolvedSource  // each source + the concrete skills chosen from it
	Targets map[string]string // agent name -> relative skills dir
	Root    string            // project root; target paths join here
	Lock    *project.Lock     // current lock (managed set); never nil
	Missing []Missing         // selectors that matched nothing (warn, not fatal)
	Scope   Scope             // which skills may be reconciled; zero value = all
}

// Scope limits a reconcile to a set of concrete skill names. The zero value is
// unrestricted (every skill is in scope). A restricted scope freezes every
// other skill: no installs, updates, or prunes, and its current lock entry is
// preserved verbatim. ScopeTo with no names restricts to nothing — patterns
// that matched nothing must freeze everything, not fall back to a full run.
type Scope struct {
	restricted bool
	names      map[string]bool
}

// ScopeTo returns a Scope restricted to exactly the given skill names.
func ScopeTo(names ...string) Scope {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return Scope{restricted: true, names: set}
}

// Includes reports whether the named skill may be reconciled.
func (s Scope) Includes(name string) bool {
	return !s.restricted || s.names[name]
}

// ResolvedSource is one manifest source resolved to a checkout commit plus the
// skills selected from it (each carrying its AbsDir + tree Hash).
type ResolvedSource struct {
	Key, Repo, Ref, Commit string
	Skills                 []*skill.Skill
}

// Missing is a selector that matched no skill in its source.
type Missing struct{ Name, Source string }

// ActionKind is one skill's fate in one target.
type ActionKind string

const (
	Install ActionKind = "install" // desired, no copy in this target yet
	Update  ActionKind = "update"  // desired, on-disk copy differs from source
	Prune   ActionKind = "prune"   // managed but no longer desired
	Noop    ActionKind = "noop"    // desired, present, up to date
)

// Action is the reconcile verdict for one (skill, target) pair. Path is the
// vendored skill DIRECTORY.
type Action struct {
	Skill  string
	Source string // origin source Key (empty for prune)
	Target string
	Path   string // absolute destination skill dir: <root>/<targetRel>/<name>
	Kind   ActionKind
}

// Plan is the full diff across every (skill, target), the lock to persist, and
// the unmatched selectors.
type Plan struct {
	Actions  []Action
	NextLock *project.Lock
	Missing  []Missing
}

// Changes reports whether the plan does anything beyond noops.
func (p *Plan) Changes() bool {
	for _, a := range p.Actions {
		if a.Kind != Noop {
			return true
		}
	}
	return false
}

// Count returns how many actions have the given kind.
func (p *Plan) Count(k ActionKind) int {
	n := 0
	for _, a := range p.Actions {
		if a.Kind == k {
			n++
		}
	}
	return n
}

// destPath is where a skill is materialized in a target: <root>/<targetRel>/<name>.
func (r Request) destPath(targetRel, name string) string {
	return filepath.Join(r.Root, targetRel, name)
}

// CollisionError reports two sources providing the same vendored skill name.
type CollisionError struct{ Name, SourceA, RepoA, SourceB, RepoB string }

func (e *CollisionError) Error() string {
	return fmt.Sprintf("skill %q is provided by both source %q (%s) and %q (%s) — keep it in only one",
		e.Name, e.SourceA, e.RepoA, e.SourceB, e.RepoB)
}

// desiredSkill ties a chosen skill to the source it came from.
type desiredSkill struct {
	skill *skill.Skill
	src   ResolvedSource
}

// buildDesired merges every source's chosen skills into a name->{skill,source}
// map, failing loud when two sources contribute the same name.
func buildDesired(sources []ResolvedSource) (map[string]desiredSkill, error) {
	desired := map[string]desiredSkill{}
	for _, src := range sources {
		for _, s := range src.Skills {
			if prev, ok := desired[s.Name]; ok {
				return nil, &CollisionError{
					Name:    s.Name,
					SourceA: prev.src.Key, RepoA: prev.src.Repo,
					SourceB: src.Key, RepoB: src.Repo,
				}
			}
			desired[s.Name] = desiredSkill{skill: s, src: src}
		}
	}
	return desired, nil
}
