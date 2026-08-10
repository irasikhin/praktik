package library

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// looksRemote reports whether a source string should be fetched with git rather
// than read as a local skills dir. A scheme or scp-style prefix is conclusive; a
// filesystem-looking path is remote only if it is itself a git repo (a local
// bare/clone); otherwise a host/path form (e.g. github.com/irasikhin/praktik-skills) is
// treated as a remote.
func looksRemote(s string) bool {
	if strings.Contains(s, "://") || strings.HasPrefix(s, "git@") {
		return true
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~") {
		return isGitRepo(s)
	}
	first, _, ok := strings.Cut(s, "/")
	return ok && strings.Contains(first, ".")
}

// isGitRepo reports whether path is a git repository (working clone or bare).
func isGitRepo(path string) bool {
	return isDir(filepath.Join(path, ".git")) || isDir(filepath.Join(path, "objects"))
}

// gitURL maps a source string to a clonable URL: schemes, scp-style, and
// filesystem paths pass through; a bare host/path gets an https:// prefix.
func gitURL(repo string) string {
	if strings.Contains(repo, "://") || strings.HasPrefix(repo, "git@") ||
		strings.HasPrefix(repo, "/") || strings.HasPrefix(repo, ".") || strings.HasPrefix(repo, "~") {
		return repo
	}
	return "https://" + repo
}

func cacheRoot() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "praktik")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "praktik")
}

// gitCacheDir is the per-(repo,ref) clone location under the cache root.
func gitCacheDir(repo, ref string) string {
	san := strings.NewReplacer("/", "_", ":", "_", "@", "_", "~", "_").Replace(repo)
	return filepath.Join(cacheRoot(), san+"@"+ref)
}

// resolveGit resolves repo at ref via the cache (cloning on first use) and
// returns its skills/ dir, capturing the checked-out commit for the lock. With
// refresh it fetches an already-cached clone first (the `praktik update` path).
func resolveGit(repo, ref string, refresh bool) (*Library, error) {
	dir, commit, err := gitEnsure(repo, ref, refresh)
	if err != nil {
		return nil, err
	}
	d := skillsUnder(dir)
	if d == "" {
		return nil, fmt.Errorf("source %s@%s has no skills/ dir", repo, ref)
	}
	return &Library{dir: d, commit: commit}, nil
}

// gitEnsure ensures a cached checkout of repo at ref and returns the checkout
// dir plus its HEAD commit. A missing cache is cloned; an existing cache is used
// as-is — no network — unless refresh is set, which fetches ref and moves the
// checkout to its latest commit. Shared by resolveGit (skills/ libraries) and
// ResolveCheckout/RefreshCheckout (recursive multi-source discovery).
func gitEnsure(repo, ref string, refresh bool) (dir, commit string, err error) {
	if ref == "" {
		ref = "main"
	}
	if _, e := exec.LookPath("git"); e != nil {
		return "", "", fmt.Errorf("git not found on PATH (needed to resolve source %q)", repo)
	}
	dir = gitCacheDir(repo, ref)
	url := gitURL(repo)
	if isDir(filepath.Join(dir, ".git")) {
		if refresh {
			if e := gitUpdate(dir, ref); e != nil {
				return "", "", fmt.Errorf("update %s@%s: %w", repo, ref, e)
			}
		}
	} else {
		if e := os.MkdirAll(filepath.Dir(dir), 0o755); e != nil {
			return "", "", e
		}
		_ = os.RemoveAll(dir)
		if e := gitClone(url, ref, dir); e != nil {
			return "", "", fmt.Errorf("clone %s@%s: %w", repo, ref, e)
		}
	}
	out, e := gitOutput(dir, "rev-parse", "HEAD")
	if e != nil {
		return "", "", e
	}
	return dir, strings.TrimSpace(out), nil
}

// gitClone tries a shallow branch/tag clone, then falls back to a full clone +
// checkout (for a commit sha, or a server that rejects --branch).
func gitClone(url, ref, dir string) error {
	if err := gitRun("", "clone", "--quiet", "--depth", "1", "--branch", ref, url, dir); err == nil {
		return nil
	}
	_ = os.RemoveAll(dir)
	if err := gitRun("", "clone", "--quiet", url, dir); err != nil {
		return err
	}
	return gitRun(dir, "checkout", "--quiet", ref)
}

// gitUpdate fetches ref and moves the working tree to its latest commit, so a
// branch advances to the remote tip and a tag/sha checks out exactly.
func gitUpdate(dir, ref string) error {
	if err := gitRun(dir, "fetch", "--quiet", "--depth", "1", "origin", ref); err != nil {
		if err2 := gitRun(dir, "fetch", "--quiet", "--all", "--tags", "--prune"); err2 != nil {
			return err
		}
	}
	if err := gitRun(dir, "checkout", "--quiet", "--detach", "FETCH_HEAD"); err == nil {
		return nil
	}
	return gitRun(dir, "checkout", "--quiet", ref)
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	return string(out), err
}
