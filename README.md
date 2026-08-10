# praktik

[![CI](https://github.com/irasikhin/praktik/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/irasikhin/praktik/actions/workflows/ci.yml?query=branch%3Amain)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A self-contained, general-purpose CLI for vendoring **agent skills** (the open
[Agent Skills](https://agentskills.io) `SKILL.md` format agents discover under
`.claude/skills`, `.agents/skills`, and friends) from **any git repo — or several
at once**, Maven-style: declare your skill sources, pick skills by pattern or
direct path, and `praktik sync` materializes them as real, committed directories
into every agent target.

Skills are third-party instructions your agents execute, yet the Agent Skills
spec leaves distribution unclaimed
([agentskills#243](https://github.com/agentskills/agentskills/discussions/243))
and mainstream installers record a lock they cannot restore from
([vercel-labs/skills#283](https://github.com/vercel-labs/skills/issues/283)).
praktik treats a skill like any other dependency — `npm ci` semantics: a
declarative manifest, a lockfile pinning each skill to its source commit **and**
content tree hash, drift detection (`status` / `diff`) when a vendored copy is
edited by hand, and upstream changes arriving only through an explicit `praktik
update`. Vendored copies are plain committed files, so every skill change is
reviewable in a PR.

Originally built for the
[praktik-skills](https://github.com/irasikhin/praktik-skills) library; it also replaces the
`praxis-skills` bash wrapper, the external `skillshare` binary, and the
`validate-skills.py` / `gen-handbook.py` scripts.

> ⚠️ **Pre-1.0.** CLI flags, the `praktik.toml` schema, and the on-disk
> layout may change between minor versions until 1.0.

Modeled on [`kpass`](https://github.com/irasikhin/kpass): alecthomas/kong, a
custom color layer, `--json` on every command, and a clean split between the CLI
and the core library/skill packages.

## Install

```bash
nix profile install github:irasikhin/praktik              # Nix (bundles git, fzf)
go install github.com/irasikhin/praktik/cmd/praktik@latest # Go
```

Or grab a [pre-built binary](https://github.com/irasikhin/praktik/releases).
Outside Nix, have `git` on PATH (source cloning) and `fzf` for `praktik pick`.

## Status

Feature-complete against `docs/design.md` — the full lifecycle plus authoring &
discovery:

**Consumption** (project by default; `-g` for the global `$HOME` scope):

| Command | Does |
|---|---|
| `praktik init [<axis\|name>...]` | Scaffold `praktik.toml` and vendor the named skills. |
| `praktik source add <repo> [sel...]` / `rm` / `ls` | Manage the skill sources (git repos / local paths). |
| `praktik add <sel>... [--from S]` / `rm` | Edit a source's selection, then sync. |
| `praktik sync` | Reconcile every target to the manifest (idempotent). |
| `praktik update [<pat>...]` | Fetch git sources, re-resolve, and rewrite the lock. |
| `praktik status [--json]` | Drift view: `+added ~changed -pruned`, or `=ok`. |
| `praktik diff [<name>]` | Per-file diff of a drifted skill vs its source. |
| `praktik target add/rm/ls` | Manage the agent target set. |
| `praktik migrate` | Import an existing `.skillshare/` setup into `praktik.toml`. |

**Discovery & authoring:**

| Command | Does |
|---|---|
| `praktik list [--axis AXIS]` | List available skills, grouped by axis. |
| `praktik search <query>` | Match name / description / axis. |
| `praktik show <name>` | Print a skill's frontmatter and body. |
| `praktik pick [--add]` | fzf picker → show, or `--add` to the manifest. |
| `praktik new <name>` | Scaffold a `skills/<name>/SKILL.md` template. |
| `praktik doctor` | Sanity-check source, targets, lock, leftovers. |
| `praktik validate` | Lint every SKILL.md frontmatter (drop-in for `validate-skills.py`). |
| `praktik handbook [--check]` | Regenerate `HANDBOOK.md` (drop-in for `gen-handbook.py`). |
| `praktik completion <shell>` | bash / zsh / fish completion. |

A project declares its skill **sources** and selections in `praktik.toml`; `praktik
sync` resolves each source (cloning git remotes on demand), discovers its skills
(any directory with a valid `SKILL.md`), and vendors the selected ones as **whole
real directories** (no symlinks, no temp paths) into every agent target. Two
sources providing the same skill name is a hard error (never a silent shadow).
`.praktik/lock.json` pins each skill to its origin source + commit + tree hash for
drift detection and reproducible resync. Resolution is cache-first: a git source
is cloned once into `~/.cache/praktik` and then reused as-is — only `praktik
update` fetches the remotes to pick up new commits. `validate` and `handbook`
reproduce the python scripts byte-for-byte, so they are verified drop-ins for
the praktik repo's CI and pre-commit hook.

A repo's agent doc (`CLAUDE.md` or `AGENTS.md` at the project root) can opt into
a **tool-managed skill list**: put a `<!-- praktik:skills BEGIN -->` /
`<!-- praktik:skills END -->` marker pair in it and every reconcile (`init`,
`add`, `rm`, `sync`, `update`, `migrate`, …) rewrites the lines between them to
the vendored skill set — the doc the agent actually loads never drifts from
`praktik.toml`. `status` and `doctor` flag a stale block; no markers, no writes.

```toml
# praktik.toml — skills from several repos, Maven-style
[targets]
claude = ".claude/skills"
# agents = ".agents/skills"   # vendor-neutral dir (Copilot, OpenHands, …)

[[source]]
name = "praktik"
repo = "github.com/irasikhin/praktik-skills"
ref  = "v1.0.0"
skills = ["proc", "lang-go"]            # axis / glob / exact name

[[source]]
repo = "github.com/owner/agent-skills"
skills = ["*", "path:experimental/rag"] # glob + a direct path
```

```sh
cd my-project
praktik init lang-shell-portable proc                 # source = $PRAKTIK_DIR / --source
praktik source add github.com/owner/agent-skills "*"  # add another source
praktik add build-maven --from praktik                 # add a selector to a named source
praktik status                                         # +added ~changed -pruned / =ok
praktik -g init praktik-index                           # or vendor globally into $HOME
```

## Locating the library

In precedence order: the `--source` flag (a local praktik repo / its `skills/`
dir, **or a git remote** cloned on demand at `--ref` into `~/.cache/praktik`),
then the manifest's `[source]`, then `$PRAKTIK_DIR/skills`, `$PRAKTIK_SKILLS_DIR`,
or a `skills/` directory in an ancestor of the current directory.

```sh
export PRAKTIK_DIR=~/src/praktik-skills  # local library checkout
praktik list

# or pin a project to a git remote at a tag:
praktik --source github.com/irasikhin/praktik-skills --ref v1.2.0 init proc lang-go
```

## Build

Go 1.26+. Inside the dev shell (`nix develop`) or with a Go toolchain on PATH:

```sh
go build ./cmd/praktik
go test ./...
```

Nix: `nix build` (or `nix run . -- list`); `nix flake check` is the gate.
A dependency change means recomputing `nix/package.nix`'s `vendorHash`
(set it to `lib.fakeHash`, build once, paste the reported `got:` hash).

## Releasing

Tag-driven. `scripts/release.sh` computes the next semver from Conventional
Commits since the last tag, writes the `CHANGELOG.md` section, updates
`version` in `nix/package.nix`, commits `chore(release): vX.Y.Z`, and creates
the annotated tag; `git push --follow-tags` then triggers
`.github/workflows/release.yml`, which verifies the tag against
`nix/package.nix`, cross-compiles linux/darwin × amd64/arm64 tarballs with
SHA256 checksums, and publishes the GitHub Release. See
[CONTRIBUTING.md](./CONTRIBUTING.md#releasing) for the bump rules.

## License

MIT — see [LICENSE](LICENSE).
