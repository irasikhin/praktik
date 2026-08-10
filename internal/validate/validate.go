// Package validate lints every skill's SKILL.md frontmatter. It is a faithful
// port of scripts/validate-skills.py: SKILL.md present, a terminated frontmatter
// block, a kebab-case `name:` equal to the directory, and a `description:` of
// 10..1024 characters.
package validate

import (
	"fmt"

	"github.com/irasikhin/praktik/internal/library"
)

// Issue is a single validation failure.
type Issue struct {
	Skill   string `json:"skill"`
	Message string `json:"message"`
}

// Run checks every skill directory in the library and returns the failures plus
// the number of directories checked.
func Run(lib *library.Library) ([]Issue, int, error) {
	dirs, err := lib.Dirs()
	if err != nil {
		return nil, 0, err
	}
	if len(dirs) == 0 {
		return nil, 0, fmt.Errorf("no skills found under %s", lib.Dir())
	}
	var issues []Issue
	for _, name := range dirs {
		s, ok, err := lib.ReadSkill(name)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			issues = append(issues, Issue{name, "missing SKILL.md"})
			continue
		}
		for _, msg := range s.Issues() {
			issues = append(issues, Issue{name, msg})
		}
	}
	return issues, len(dirs), nil
}
