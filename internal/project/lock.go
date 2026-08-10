package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// Lock is the machine-written resolved state in .praktik/lock.json. Its Skills map
// IS the managed set: sync only ever prunes entries it recorded here. Each entry
// pins the skill's origin source + commit + tree hash for reproducible resync.
type Lock struct {
	Sources []LockSource           `json:"sources"`
	Skills  map[string]LockedSkill `json:"skills"`
}

// LockSource records a resolved source and the commit it was resolved at.
type LockSource struct {
	Name   string `json:"name,omitempty"`
	Repo   string `json:"repo"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

// LockedSkill pins one vendored skill to its origin source and its tree hash.
type LockedSkill struct {
	Source string `json:"source,omitempty"`
	Repo   string `json:"repo,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Commit string `json:"commit,omitempty"`
	Hash   string `json:"hash"`
}

// rawLock tolerates the older single-`source` object shape on read.
type rawLock struct {
	Source  *LockSource            `json:"source"`
	Sources []LockSource           `json:"sources"`
	Skills  map[string]LockedSkill `json:"skills"`
}

// LoadLock reads .praktik/lock.json. A missing file yields an empty lock (first
// sync); an older object-shaped `source` is folded into the sources list and the
// next sync rewrites it.
func LoadLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Lock{Skills: map[string]LockedSkill{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var rl rawLock
	if err := json.Unmarshal(data, &rl); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	l := &Lock{Sources: rl.Sources, Skills: rl.Skills}
	if len(l.Sources) == 0 && rl.Source != nil {
		l.Sources = []LockSource{*rl.Source}
	}
	if l.Skills == nil {
		l.Skills = map[string]LockedSkill{}
	}
	return l, nil
}

// SaveLock atomically writes the lock as indented JSON.
func SaveLock(path string, l *Lock) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return WriteAtomic(path, data, 0o644)
}

// Managed returns the sorted names of the managed skill set.
func (l *Lock) Managed() []string {
	names := make([]string, 0, len(l.Skills))
	for n := range l.Skills {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
