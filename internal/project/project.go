// Package project owns a praktik project's on-disk state: the human-edited
// praktik.toml manifest (desired skills + agent targets + source) and the
// machine-written .praktik/lock.json (the resolved managed set + content hashes).
// It replaces the .skillshare/config.yaml the old skillshare wrapper used, and
// knows nothing about the library or the reconcile engine — it just locates,
// loads, and atomically saves the two files.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ManifestName is the project manifest filename, found at the project root.
const ManifestName = "praktik.toml"

// LegacyManifestName is the pre-rename manifest filename. Discovery still
// honors it (with a legacy-flagged handle) so old projects keep working;
// `praktik migrate` renames the state to the praktik names.
const LegacyManifestName = "praxis.toml"

// ErrNoManifest is returned by LoadManifest when the file is absent, so callers
// can distinguish "no project here" from a corrupt manifest.
var ErrNoManifest = errors.New("no praktik.toml found")

// Project is a resolved scope to vendor into: a manifest file, a lock file, and
// the root that target paths are relative to. Project scope keeps the manifest at
// <root>/praktik.toml and the lock under <root>/.praktik/; global scope (Global)
// points them at the XDG config/state dirs with the root at $HOME.
type Project struct {
	root         string
	manifestPath string
	lockPath     string
	legacy       bool
}

// Root is the directory target paths are joined against.
func (p *Project) Root() string { return p.root }

// Legacy reports whether this handle points at pre-rename praxis-named state
// (praxis.toml / .praxis, or the old global config); such projects work but
// should be renamed with `praktik migrate`.
func (p *Project) Legacy() bool { return p.legacy }

// ManifestPath is the absolute path to the manifest (praktik.toml or, for global
// scope, the config.toml).
func (p *Project) ManifestPath() string { return p.manifestPath }

// LockPath is the absolute path to the resolved lock file.
func (p *Project) LockPath() string { return p.lockPath }

// At returns a project-scope handle rooted exactly at dir, without requiring
// praktik.toml to exist yet. Used by `init`.
func At(dir string) *Project {
	d := abs(dir)
	return &Project{
		root:         d,
		manifestPath: filepath.Join(d, ManifestName),
		lockPath:     filepath.Join(d, ".praktik", "lock.json"),
	}
}

// Find walks up from startDir looking for a praktik.toml (or, failing that at
// the same level, a legacy praxis.toml). ok is false (with a nil error) when
// none is found up to the filesystem root.
func Find(startDir string) (p *Project, ok bool, err error) {
	dir := abs(startDir)
	for {
		if fileExists(filepath.Join(dir, ManifestName)) {
			return At(dir), true, nil
		}
		if fileExists(filepath.Join(dir, LegacyManifestName)) {
			return &Project{
				root:         dir,
				manifestPath: filepath.Join(dir, LegacyManifestName),
				lockPath:     filepath.Join(dir, ".praxis", "lock.json"),
				legacy:       true,
			}, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, false, nil
		}
		dir = parent
	}
}

// Global returns the global-scope handle: the manifest is
// $XDG_CONFIG_HOME/praktik/config.toml, the lock is $XDG_STATE_HOME/praktik/
// lock.json, and skills vendor into $HOME-relative targets.
func Global() (*Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	p := &Project{
		root:         home,
		manifestPath: filepath.Join(configHome(home), "praktik", "config.toml"),
		lockPath:     filepath.Join(stateHome(home), "praktik", "lock.json"),
	}
	if !fileExists(p.manifestPath) {
		legacy := &Project{
			root:         home,
			manifestPath: filepath.Join(configHome(home), "praxis", "config.toml"),
			lockPath:     filepath.Join(stateHome(home), "praxis", "lock.json"),
			legacy:       true,
		}
		if fileExists(legacy.manifestPath) {
			return legacy, nil
		}
	}
	return p, nil
}

// MigrateProjectState renames legacy praxis-named project state under root
// (praxis.toml and .praxis/) to the praktik names, returning a human-readable
// list of the moves it made. Both-names-exist is an error, never a merge.
func MigrateProjectState(root string) (moved []string, err error) {
	root = abs(root)
	renames := [][2]string{
		{filepath.Join(root, LegacyManifestName), filepath.Join(root, ManifestName)},
		{filepath.Join(root, ".praxis"), filepath.Join(root, ".praktik")},
	}
	return applyRenames(renames)
}

// MigrateGlobalState renames the legacy global config/state files
// (~/.config/praxis/config.toml, ~/.local/state/praxis/lock.json) to their
// praktik locations, removing the emptied legacy dirs.
func MigrateGlobalState() (moved []string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	renames := [][2]string{
		{filepath.Join(configHome(home), "praxis", "config.toml"), filepath.Join(configHome(home), "praktik", "config.toml")},
		{filepath.Join(stateHome(home), "praxis", "lock.json"), filepath.Join(stateHome(home), "praktik", "lock.json")},
	}
	moved, err = applyRenames(renames)
	if err == nil {
		_ = os.Remove(filepath.Join(configHome(home), "praxis"))
		_ = os.Remove(filepath.Join(stateHome(home), "praxis"))
	}
	return moved, err
}

func applyRenames(renames [][2]string) (moved []string, err error) {
	for _, r := range renames {
		old, nw := r[0], r[1]
		if !fileExists(old) {
			continue
		}
		if fileExists(nw) {
			return moved, fmt.Errorf("both %s and %s exist — remove one and retry", old, nw)
		}
		if err := os.MkdirAll(filepath.Dir(nw), 0o755); err != nil {
			return moved, err
		}
		if err := os.Rename(old, nw); err != nil {
			return moved, err
		}
		moved = append(moved, old+" -> "+nw)
	}
	return moved, nil
}

func configHome(home string) string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}

func stateHome(home string) string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".local", "state")
}

// Manifest is the human-edited desired state in praktik.toml: a set of skill
// sources (git repos / local paths, each with its own selection) and the agent
// targets to vendor into.
type Manifest struct {
	Sources []Source          `toml:"source"`  // [[source]] array
	Targets map[string]string `toml:"targets"` // agent name -> relative skills dir
}

// Source is one skill dependency: a repo (local path or git remote, empty =
// ambient $PRAKTIK_DIR/ancestor), an optional ref + discovery root, and the
// selectors picking skills from it (axis/glob/name, or path:<repo-rel-dir>).
type Source struct {
	Name   string   `toml:"name,omitempty"` // optional alias; identity falls back to Repo
	Repo   string   `toml:"repo,omitempty"`
	Ref    string   `toml:"ref,omitempty"`
	Root   string   `toml:"root,omitempty"`
	Skills []string `toml:"skills"`
}

// Key is the source's identity for the lock, collisions, and CLI selection: the
// alias if set, else the repo (else "<ambient>").
func (s Source) Key() string {
	switch {
	case s.Name != "":
		return s.Name
	case s.Repo != "":
		return s.Repo
	default:
		return "<ambient>"
	}
}

// PrimarySource returns (creating if absent) the first source — the implicit
// target for single-source projects and `init`.
func (m *Manifest) PrimarySource() *Source {
	if len(m.Sources) == 0 {
		m.Sources = []Source{{Skills: []string{}}}
	}
	return &m.Sources[0]
}

// DefaultManifest is the manifest a fresh `praktik init` starts from: the claude
// target and one empty (ambient) source.
func DefaultManifest() *Manifest {
	return &Manifest{
		Sources: []Source{{Skills: []string{}}},
		Targets: map[string]string{"claude": ".claude/skills"},
	}
}

// rawManifest captures the manifest with `source` left undecoded, so LoadManifest
// can branch on whether it is a single [source] table (legacy) or a [[source]]
// array.
type rawManifest struct {
	Source  toml.Primitive    `toml:"source"`
	Targets map[string]string `toml:"targets"`
	Skills  []string          `toml:"skills"` // legacy top-level selection
}

// LoadManifest reads and decodes praktik.toml. It returns ErrNoManifest when the
// file does not exist. A legacy single `[source]` table (+ top-level `skills`)
// is folded into one Source; the new `[[source]]` array decodes directly.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoManifest
	}
	if err != nil {
		return nil, err
	}

	var raw rawManifest
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	m := &Manifest{Targets: raw.Targets}
	switch md.Type("source") {
	case "ArrayHash": // [[source]]
		if err := md.PrimitiveDecode(raw.Source, &m.Sources); err != nil {
			return nil, fmt.Errorf("parse %s [[source]]: %w", path, err)
		}
	case "Hash": // legacy single [source]
		var s Source
		if err := md.PrimitiveDecode(raw.Source, &s); err != nil {
			return nil, fmt.Errorf("parse %s [source]: %w", path, err)
		}
		s.Skills = append(s.Skills, raw.Skills...)
		m.Sources = []Source{s}
	default: // no [source]; legacy top-level skills → one ambient source
		if len(raw.Skills) > 0 {
			m.Sources = []Source{{Skills: raw.Skills}}
		}
	}

	normalizeManifest(m)
	if err := checkDupSources(m); err != nil {
		return nil, err
	}
	return m, nil
}

func normalizeManifest(m *Manifest) {
	if m.Targets == nil {
		m.Targets = map[string]string{}
	}
	for i := range m.Sources {
		if m.Sources[i].Skills == nil {
			m.Sources[i].Skills = []string{}
		}
	}
}

func checkDupSources(m *Manifest) error {
	seen := map[string]bool{}
	for _, s := range m.Sources {
		if seen[s.Key()] {
			return fmt.Errorf("duplicate source %q in praktik.toml", s.Key())
		}
		seen[s.Key()] = true
	}
	return nil
}

// SaveManifest atomically writes the manifest as TOML (canonical [[source]] form,
// so a loaded legacy manifest is rewritten on the next save).
func SaveManifest(path string, m *Manifest) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	return WriteAtomic(path, buf.Bytes(), 0o644)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
