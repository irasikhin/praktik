// Package skill models a single praktik skill — one skills/<name>/SKILL.md file:
// its YAML frontmatter (name, description, metadata.axis), markdown body, and a
// content hash. The frontmatter reader mirrors the stdlib-only python parser in
// scripts/validate-skills.py / scripts/gen-handbook.py exactly, so existing
// skills parse and validate identically.
package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Description length bounds, matching validate-skills.py.
const (
	DescMin = 10
	DescMax = 1024
)

var (
	kebabRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	axisRe  = regexp.MustCompile(`axis:\s*([a-z0-9-]+)`)
)

// Skill is one parsed SKILL.md, optionally with its directory provenance.
type Skill struct {
	Name        string // directory name — the identity
	FrontName   string // frontmatter `name:` (validated against Name)
	Description string // frontmatter `description:` (folded/multiline joined)
	Axis        string // metadata.axis, or "other" when absent
	Body        string // markdown after the frontmatter block
	HasFront    bool   // a well-formed --- ... --- block was present
	Raw         []byte // the SKILL.md file bytes
	Hash        string // content address for the lock: sha256(Raw), or the dir tree hash when AbsDir is set

	RelDir string // source-relative skill directory (e.g. "skills/lang-java-naming"); empty for byte-only parses
	AbsDir string // absolute skill directory on the resolved checkout
}

// IsKebab reports whether s is a kebab-case identifier.
func IsKebab(s string) bool { return kebabRe.MatchString(s) }

// Issues returns the frontmatter validation failures for this skill, as the
// validator (internal/validate) and discovery both apply them — a single
// definition of "a valid skill".
func (s *Skill) Issues() []string {
	if !s.HasFront {
		return []string{"missing or unterminated --- frontmatter ---"}
	}
	var out []string
	if s.FrontName == "" {
		out = append(out, "frontmatter has no `name:`")
	} else {
		if !IsKebab(s.FrontName) {
			out = append(out, fmt.Sprintf("name '%s' is not kebab-case", s.FrontName))
		}
		if s.FrontName != s.Name {
			out = append(out, fmt.Sprintf("name '%s' != directory '%s'", s.FrontName, s.Name))
		}
	}
	if s.Description == "" {
		out = append(out, "frontmatter has no `description:`")
	} else if n := len([]rune(s.Description)); n < DescMin || n > DescMax {
		out = append(out, fmt.Sprintf("description length %d outside [%d,%d]", n, DescMin, DescMax))
	}
	return out
}

// Valid reports whether the skill passes every frontmatter rule.
func (s *Skill) Valid() bool { return len(s.Issues()) == 0 }

// ParseDir loads a skill from a directory: it reads SKILL.md, parses the
// frontmatter, records the directory, and sets Hash to a tree hash over the whole
// directory (so supporting files are part of the content address).
func ParseDir(absDir, relDir string) (*Skill, error) {
	content, err := os.ReadFile(filepath.Join(absDir, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	s := Parse(filepath.Base(absDir), content)
	s.RelDir = relDir
	s.AbsDir = absDir
	th, err := TreeHash(absDir)
	if err != nil {
		return nil, err
	}
	s.Hash = th
	return s, nil
}

// TreeHash is a deterministic sha256 over a directory's regular files: walk
// (skipping .git), record forward-slash relative paths, sort them, then feed
// "<rel>\x00<len>\x00<bytes>" per file into one hash. Stable across machines, so
// it is the content address for whole-directory vendoring.
func TreeHash(absDir string) (string, error) {
	files := map[string][]byte{}
	var rels []string
	err := filepath.WalkDir(absDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(absDir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		rels = append(rels, rel)
		files[rel] = data
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rels)
	h := sha256.New()
	for _, rel := range rels {
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(files[rel]))
		h.Write(files[rel])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Parse reads SKILL.md bytes for the skill whose directory is name.
func Parse(name string, content []byte) *Skill {
	s := &Skill{Name: name, Raw: content, Hash: hashOf(content)}
	front, body, ok := split(string(content))
	s.Body = body
	if !ok {
		return s
	}
	s.HasFront = true
	s.FrontName = fieldValue(front, "name")
	s.Description = fieldValue(front, "description")
	s.Axis = axisOf(front)
	return s
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// split returns the frontmatter body between the first two `---` fences and the
// remaining markdown. ok is false when there is no terminated frontmatter.
func split(text string) (front, body string, ok bool) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", text, false
	}
	for idx := 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "---" {
			return strings.Join(lines[1:idx], "\n"), strings.Join(lines[idx+1:], "\n"), true
		}
	}
	return "", text, false
}

// fieldValue reads a top-level scalar or folded/literal block value for key,
// matching the python field_value reader.
func fieldValue(front, key string) string {
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `:\s*(.*)$`)
	lines := strings.Split(front, "\n")
	for idx := 0; idx < len(lines); idx++ {
		m := re.FindStringSubmatch(lines[idx])
		if m == nil {
			continue
		}
		inline := strings.TrimSpace(m[1])
		if inline != "" && inline != ">" && inline != "|" && inline != ">-" && inline != "|-" {
			return strings.Trim(inline, "'\"")
		}
		// Folded/literal block: gather the following more-indented lines until a
		// line starting in column 0 (the next top-level key) ends the block.
		var parts []string
		for _, cont := range lines[idx+1:] {
			if strings.TrimSpace(cont) == "" {
				continue
			}
			if cont[0] != ' ' && cont[0] != '\t' {
				break
			}
			parts = append(parts, strings.TrimSpace(cont))
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	}
	return ""
}

func axisOf(front string) string {
	m := axisRe.FindStringSubmatch(front)
	if m == nil {
		return "other"
	}
	return m[1]
}
