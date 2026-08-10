package sync

import (
	"os"
	"sort"

	"github.com/irasikhin/praktik/internal/project"
	"github.com/irasikhin/praktik/internal/skill"
)

// Compute computes the reconcile diff. It merges the selected skills across
// sources (failing on a name collision), then for every desired skill × target
// classifies install/update/noop by comparing the source tree hash against the
// hash of the on-disk vendored directory — so a copy whose files were edited
// (drift) is detected as Update. Managed skills no longer desired are pruned.
// A restricted req.Scope freezes every out-of-scope skill: no actions and, when
// it is already locked, its lock entry carried over verbatim.
func Compute(req Request) (*Plan, error) {
	desired, err := buildDesired(req.Sources)
	if err != nil {
		return nil, err
	}
	next := &project.Lock{
		Sources: lockSources(req.Sources),
		Skills:  map[string]project.LockedSkill{},
	}
	p := &Plan{NextLock: next, Missing: req.Missing}
	if err := planDesired(req, p, desired); err != nil {
		return nil, err
	}
	planPrunes(req, p, desired)
	sortActions(p.Actions)
	return p, nil
}

// planDesired locks every desired in-scope skill at its source state and
// classifies it across targets. An out-of-scope desired skill is frozen: a
// not-yet-locked one is neither installed nor locked, a locked one keeps its
// existing lock entry verbatim.
func planDesired(req Request, p *Plan, desired map[string]desiredSkill) error {
	targets := sortedKeys(req.Targets)
	for _, name := range sortedDesired(desired) {
		if !req.Scope.Includes(name) {
			preserveLocked(req, p, name)
			continue
		}
		d := desired[name]
		p.NextLock.Skills[name] = project.LockedSkill{
			Source: d.src.Key, Repo: d.src.Repo, Ref: d.src.Ref, Commit: d.src.Commit, Hash: d.skill.Hash,
		}
		for _, t := range targets {
			dest := req.destPath(req.Targets[t], name)
			kind, err := classifyDir(dest, d.skill.Hash)
			if err != nil {
				return err
			}
			p.Actions = append(p.Actions, Action{Skill: name, Source: d.src.Key, Target: t, Path: dest, Kind: kind})
		}
	}
	return nil
}

// planPrunes prunes managed skills no longer desired. An out-of-scope one is
// frozen: kept on disk and in the lock verbatim.
func planPrunes(req Request, p *Plan, desired map[string]desiredSkill) {
	targets := sortedKeys(req.Targets)
	for _, name := range req.Lock.Managed() {
		if _, ok := desired[name]; ok {
			continue
		}
		if !req.Scope.Includes(name) {
			preserveLocked(req, p, name)
			continue
		}
		for _, t := range targets {
			dest := req.destPath(req.Targets[t], name)
			if dirExists(dest) {
				p.Actions = append(p.Actions, Action{Skill: name, Target: t, Path: dest, Kind: Prune})
			}
		}
	}
}

// preserveLocked carries a frozen skill's existing lock entry into the next
// lock verbatim — hash and source attribution untouched even when the source
// moved — so a later full reconcile still sees it as managed.
func preserveLocked(req Request, p *Plan, name string) {
	if prev, ok := req.Lock.Skills[name]; ok {
		p.NextLock.Skills[name] = prev
	}
}

// classifyDir decides install/update/noop for one destination directory against
// the source tree hash: absent => install, equal => noop, different => update.
func classifyDir(dest, sourceHash string) (ActionKind, error) {
	if !dirExists(dest) {
		return Install, nil
	}
	h, err := skill.TreeHash(dest)
	if err != nil {
		return "", err
	}
	if h == sourceHash {
		return Noop, nil
	}
	return Update, nil
}

func lockSources(sources []ResolvedSource) []project.LockSource {
	out := make([]project.LockSource, 0, len(sources))
	for _, s := range sources {
		name := ""
		if s.Key != s.Repo {
			name = s.Key
		}
		out = append(out, project.LockSource{Name: name, Repo: s.Repo, Ref: s.Ref, Commit: s.Commit})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortedDesired(desired map[string]desiredSkill) []string {
	names := make([]string, 0, len(desired))
	for n := range desired {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func sortActions(a []Action) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].Target != a[j].Target {
			return a[i].Target < a[j].Target
		}
		return a[i].Skill < a[j].Skill
	})
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
