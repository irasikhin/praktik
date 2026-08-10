package library

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/irasikhin/praktik/internal/skill"
)

// Checkout is a resolved source repo root (not its skills/ dir) plus the commit
// it was resolved at — the unit a multi-source manifest discovers skills under.
type Checkout struct {
	dir    string
	commit string
}

// Dir is the absolute checkout root.
func (c *Checkout) Dir() string { return c.dir }

// Commit is the resolved commit, or "local" for a non-git source.
func (c *Checkout) Commit() string { return c.commit }

// ResolveCheckout resolves a source to its repo checkout root: a local path is
// used directly (commit "local"); a git remote is cloned on demand at ref into
// ~/.cache/praktik (commit = HEAD), and an existing cache is used as-is — never
// fetched. An empty repo falls back to $PRAKTIK_DIR or an ancestor that contains
// a skills/ dir, matching Resolve's ambient precedence.
func ResolveCheckout(repo, ref string) (*Checkout, error) {
	return resolveCheckout(repo, ref, false)
}

// RefreshCheckout is ResolveCheckout with a forced fetch: an already-cached git
// source is fetched and moved to the latest commit of ref before resolving —
// the `praktik update` path. Local-path and ambient sources are unaffected.
func RefreshCheckout(repo, ref string) (*Checkout, error) {
	return resolveCheckout(repo, ref, true)
}

func resolveCheckout(repo, ref string, refresh bool) (*Checkout, error) {
	if repo == "" {
		if v := os.Getenv("PRAKTIK_DIR"); v != "" && isDir(v) {
			return &Checkout{dir: abs(v), commit: "local"}, nil
		}
		if cwd, err := os.Getwd(); err == nil {
			for dir := cwd; ; {
				cand := filepath.Join(dir, "skills")
				if isDir(cand) && looksLikeSkills(cand) {
					return &Checkout{dir: dir, commit: "local"}, nil
				}
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				dir = parent
			}
		}
		return nil, errors.New("cannot locate a source — set PRAKTIK_DIR or give the [[source]] a repo")
	}
	if isDir(repo) && !isBareRepo(repo) {
		// A local working directory — used in place (no clone).
		return &Checkout{dir: abs(repo), commit: "local"}, nil
	}
	if looksRemote(repo) {
		// A remote URL/host-path, or a local bare repo — clone on demand.
		dir, commit, err := gitEnsure(repo, ref, refresh)
		if err != nil {
			return nil, err
		}
		return &Checkout{dir: dir, commit: commit}, nil
	}
	return nil, fmt.Errorf("source %q is neither a local directory nor a recognized git remote", repo)
}

// isBareRepo reports whether path is a bare git repository (objects/ + HEAD, no
// working tree), which must be cloned rather than used in place.
func isBareRepo(path string) bool {
	return isDir(filepath.Join(path, "objects")) && !isDir(filepath.Join(path, ".git")) && isFile(filepath.Join(path, "HEAD"))
}

// discoverExcludes are directory names never descended into during discovery.
var discoverExcludes = map[string]bool{
	".git": true, "node_modules": true, ".praktik": true, ".praxis": true,
	".cache": true, "vendor": true, ".direnv": true,
}

// scanRoot resolves the discovery base: the checkout root, or its `root` subdir.
func (c *Checkout) scanRoot(root string) (string, error) {
	base := c.dir
	if root != "" {
		base = filepath.Join(c.dir, filepath.FromSlash(root))
	}
	if !isDir(base) {
		return "", fmt.Errorf("root %q not found in source", root)
	}
	return base, nil
}

// Discover recursively finds skill directories under the checkout (or its `root`
// subdir): any directory holding a valid SKILL.md is a skill. A directory with a
// SKILL.md is claimed and not descended into (a nested SKILL.md is part of that
// skill's tree, never a second skill). Excluded dirs are skipped. Results are
// sorted by name.
func (c *Checkout) Discover(root string) ([]*skill.Skill, error) {
	base, err := c.scanRoot(root)
	if err != nil {
		return nil, err
	}
	var skills []*skill.Skill
	err = filepath.WalkDir(base, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if p != base && discoverExcludes[d.Name()] {
			return fs.SkipDir
		}
		if !isFile(filepath.Join(p, "SKILL.md")) {
			return nil
		}
		rel, err := filepath.Rel(c.dir, p)
		if err != nil {
			return err
		}
		s, err := skill.ParseDir(p, filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		if s.Valid() {
			skills = append(skills, s)
		}
		return fs.SkipDir // claimed: never recurse into a skill dir
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// SkillByPath loads the skill at a repo-relative dir (the `path:` selector),
// resolved under the `root` subdir. It rejects a path that escapes the checkout
// and a dir without a valid SKILL.md.
func (c *Checkout) SkillByPath(root, rel string) (*skill.Skill, error) {
	base, err := c.scanRoot(root)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(base, filepath.FromSlash(filepath.Clean("/"+rel)))
	if r, err := filepath.Rel(c.dir, target); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("path %q is outside the source", rel)
	}
	if !isFile(filepath.Join(target, "SKILL.md")) {
		return nil, fmt.Errorf("no SKILL.md at path %q", rel)
	}
	relToCheckout, _ := filepath.Rel(c.dir, target)
	s, err := skill.ParseDir(target, filepath.ToSlash(relToCheckout))
	if err != nil {
		return nil, err
	}
	if !s.Valid() {
		return nil, fmt.Errorf("skill at %q is invalid: %s", rel, strings.Join(s.Issues(), "; "))
	}
	return s, nil
}

// Select resolves a source's selectors against its discovered skills: a `path:`
// (or slash-bearing) selector is a direct directory (hard error if absent/invalid);
// every other token is an axis/glob/name pattern matched against discovered names.
// Patterns matching nothing are returned as missing (warned, not fatal).
func (c *Checkout) Select(root string, discovered []*skill.Skill, selectors []string) (chosen []*skill.Skill, missing []string, err error) {
	byName := make(map[string]*skill.Skill, len(discovered))
	names := make([]string, 0, len(discovered))
	for _, s := range discovered {
		byName[s.Name] = s
		names = append(names, s.Name)
	}

	picked := map[string]*skill.Skill{}
	var patterns []string
	for _, sel := range selectors {
		if isPathSelector(sel) {
			rel := strings.TrimPrefix(sel, "path:")
			s, e := c.SkillByPath(root, rel)
			if e != nil {
				return nil, nil, e
			}
			picked[s.Name] = s
			continue
		}
		patterns = append(patterns, sel)
	}
	for _, name := range Expand(names, patterns) {
		picked[name] = byName[name]
	}
	for _, pat := range patterns {
		if len(Expand(names, []string{pat})) == 0 {
			missing = append(missing, pat)
		}
	}

	for _, s := range picked {
		chosen = append(chosen, s)
	}
	sort.Slice(chosen, func(i, j int) bool { return chosen[i].Name < chosen[j].Name })
	sort.Strings(missing)
	return chosen, missing, nil
}

func isPathSelector(sel string) bool {
	return strings.HasPrefix(sel, "path:") || strings.Contains(sel, "/")
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
