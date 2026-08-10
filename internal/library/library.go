// Package library resolves and reads a praktik skill library — the skills/ dir of
// a praktik repo. It is the source-of-truth layer: locating the library, reading
// + parsing skills, and expanding axis/glob/name patterns to concrete names
// (the batch-addressing model from the praktik-skills wrapper).
package library

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/irasikhin/praktik/internal/skill"
)

// Library is a resolved skills/ directory plus the commit it was resolved at
// ("local" for a non-git source).
type Library struct {
	dir    string
	commit string
}

// Dir is the absolute path to the skills/ directory.
func (l *Library) Dir() string { return l.dir }

// Root is the repo root (the parent of skills/).
func (l *Library) Root() string { return filepath.Dir(l.dir) }

// Commit is the source commit the library was resolved at, or "local" for a
// local (non-git) source. The reconcile lock records it for reproducibility.
func (l *Library) Commit() string { return l.commit }

// HandbookPath is where the generated HANDBOOK.md lives.
func (l *Library) HandbookPath() string { return filepath.Join(l.Root(), "HANDBOOK.md") }

// Resolve locates the skill library from a single source string (the --source
// flag or env/ancestor fallback). It is ResolveSource with no explicit ref.
func Resolve(override string) (*Library, error) {
	return ResolveSource(override, "")
}

// ResolveSource locates the skill library. Precedence: an explicit repo (a local
// path or a git remote, the latter cloned on demand at ref into ~/.cache/praktik;
// an existing cache is used as-is, never fetched), then $PRAKTIK_DIR/skills, then
// $PRAKTIK_SKILLS_DIR, then a skills/ dir in an ancestor of the cwd. ref applies
// only to a git remote.
func ResolveSource(repo, ref string) (*Library, error) {
	if repo != "" {
		if d := skillsUnder(repo); d != "" {
			return &Library{dir: d, commit: "local"}, nil
		}
		if looksRemote(repo) {
			return resolveGit(repo, ref, false)
		}
		return nil, fmt.Errorf("no skills found under %s (looked for %s/skills and %s itself)", repo, repo, repo)
	}
	if v := os.Getenv("PRAKTIK_DIR"); v != "" {
		if d := filepath.Join(v, "skills"); isDir(d) {
			return &Library{dir: abs(d), commit: "local"}, nil
		}
	}
	if v := os.Getenv("PRAKTIK_SKILLS_DIR"); v != "" {
		if isDir(v) {
			return &Library{dir: abs(v), commit: "local"}, nil
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; {
			cand := filepath.Join(dir, "skills")
			if isDir(cand) && looksLikeSkills(cand) {
				return &Library{dir: cand, commit: "local"}, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return nil, errors.New("cannot locate a praktik skills dir — set PRAKTIK_DIR=/path/to/praktik or pass --source")
}

func skillsUnder(p string) string {
	if d := filepath.Join(p, "skills"); isDir(d) {
		return abs(d)
	}
	if isDir(p) && looksLikeSkills(p) {
		return abs(p)
	}
	return ""
}

func looksLikeSkills(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil {
				return true
			}
		}
	}
	return false
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// Dirs returns the sorted names of every subdirectory of the library, including
// any that lack a SKILL.md (so validate can report them).
func (l *Library) Dirs() ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadSkill parses the named skill. ok is false (with a nil error) when the
// directory has no SKILL.md.
func (l *Library) ReadSkill(name string) (*skill.Skill, bool, error) {
	p := filepath.Join(l.dir, name, "SKILL.md")
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return skill.Parse(name, data), true, nil
}

// Skills returns every skill with a SKILL.md, sorted by name.
func (l *Library) Skills() ([]*skill.Skill, error) {
	dirs, err := l.Dirs()
	if err != nil {
		return nil, err
	}
	out := make([]*skill.Skill, 0, len(dirs))
	for _, name := range dirs {
		s, ok, err := l.ReadSkill(name)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// Skill returns one skill by name, or an error if it is absent.
func (l *Library) Skill(name string) (*skill.Skill, error) {
	s, ok, err := l.ReadSkill(name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("skill %q not found in %s", name, l.dir)
	}
	return s, nil
}

// Names returns the names of every skill with a SKILL.md.
func (l *Library) Names() ([]string, error) {
	skills, err := l.Skills()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return names, nil
}

// Expand resolves axis/glob/name patterns against allNames to a sorted, deduped
// set: a bare token that exactly names a skill matches only that skill; any other
// bare token is an axis prefix (foo matches foo-*); a token containing * ? or [
// is a path.Match glob. Exact-wins keeps a skill selectable even when its name
// prefixes a sibling (tool-sops vs tool-sops-extra).
func Expand(allNames, patterns []string) []string {
	set := map[string]bool{}
	for _, pat := range patterns {
		glob := strings.ContainsAny(pat, "*?[")
		exact := false
		if !glob {
			for _, name := range allNames {
				if name == pat {
					exact = true
					break
				}
			}
		}
		for _, name := range allNames {
			switch {
			case glob:
				if ok, _ := path.Match(pat, name); ok {
					set[name] = true
				}
			case exact:
				if name == pat {
					set[name] = true
				}
			case strings.HasPrefix(name, pat+"-"):
				set[name] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
