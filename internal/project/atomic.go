package project

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path via a temp file in the same directory followed
// by a rename, so a reader never observes a half-written file and a crash leaves
// either the old file or the new one — never a truncated one. The parent
// directory is created if missing. Mirrors the temp+chmod+rename pattern in
// kpass's db.Save. The sync engine reuses this to materialize skill files.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	tmpPath = ""
	return nil
}

// MaterializeDir atomically replaces destDir with a copy of srcDir: it stages the
// copy into a temp sibling dir, then swaps it into place (renaming any existing
// dest aside, then removing it). The original dest is untouched until the final
// rename succeeds, so a failure mid-copy leaves it intact. Files get perm; dirs
// 0o755; .git is never copied.
func MaterializeDir(destDir, srcDir string, perm os.FileMode) error {
	parent := filepath.Dir(destDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(destDir)+".tmp-*")
	if err != nil {
		return err
	}
	staged := true
	defer func() {
		if staged {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := copyTree(srcDir, tmp, perm); err != nil {
		return err
	}

	var old string
	if _, err := os.Lstat(destDir); err == nil {
		// Move the existing dest aside to a fresh (non-existent) path so the
		// rename is a plain move, not a replace-empty-dir (which can EEXIST).
		old = filepath.Join(parent, "."+filepath.Base(destDir)+".old")
		_ = os.RemoveAll(old)
		if err := os.Rename(destDir, old); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, destDir); err != nil {
		if old != "" {
			_ = os.Rename(old, destDir)
		}
		return err
	}
	staged = false
	if old != "" {
		_ = os.RemoveAll(old)
	}
	return nil
}

// copyTree deep-copies srcDir's regular files into dst, recreating directories.
func copyTree(srcDir, dst string, perm os.FileMode) error {
	return filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if d.Name() == ".git" && p != srcDir {
				return fs.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, perm)
	})
}
