# praktik CLI — API & architecture design

## Context

The praktik skill library (`skills/<name>/SKILL.md` files) was originally consumed through two stacked
tools: a bash wrapper over an external Go binary (`skillshare`). This stack is fragile and awkward:

- **Dead source paths.** `skillshare` records each vendored skill's `source:` as an ephemeral temp dir
  (`/tmp/tmp.XXXXXXXXXX/<skill>`), visible in every project's `.skillshare/config.yaml`. Once the temp
  dir is GC'd, `skillshare update` can't resync — the whole reason for the wrapper's `update` rework.
- **Two-layer indirection.** `.skillshare/skills/<name>` copies → `.claude/skills/<name>` symlinks, plus
  `chmod -R u+w` hacks after every sync.
- **No drift/pin model.** Skills always track HEAD; no way to see what changed or pin a project to a tag.
- **Split-brain authoring.** Frontmatter lint (`validate-skills.py`) and handbook generation
  (`gen-handbook.py`) are separate python scripts; no single tool owns the lifecycle.

We will replace both with **one self-contained Go CLI, `praktik`**, modeled on the maintainer's `kpass`
(Go + alecthomas/kong, rich UX, three-layer split) and `sandboxer`.
This document is the **API design first** — command surface, state model, and the internal contract —
ahead of implementation.

## Decisions (locked)

- **Self-contained** — own the vendoring/sync/manifest logic; no `skillshare` dependency.
- **kpass model** — alecthomas/kong, custom `color`/`tree`/`picker` packages, `--json` on every command,
  `ctx` injection, three-layer split (CLI ↔ core ↔ storage).
- **Binary name: `praktik`** (`praktik add lang-java`, `praktik sync`).
- **Full lifecycle** — consumption + authoring (replaces the legacy wrapper, `skillshare`,
  `validate-skills.py`, `gen-handbook.py`).
- **Declarative state** — `praktik.toml` (desired) + `.praktik/lock.json` (resolved), cargo-style reconcile.
- **Separate Go repo** — the binary operates on a skill library found via `$PRAKTIK_DIR` or a git remote
  (clone-on-demand, pinnable). The existing skill repo keeps only skills and calls the binary in CI.

## Multi-source evolution (v0.2)

The sections below describe the original single-`[source]` design. praktik is now
**general-purpose and multi-source** — the shipped model:

- **`praktik.toml` holds an array of `[[source]]`** deps, each `{name?, repo, ref?,
  root?, skills=[…]}`. A legacy single `[source]` + top-level `skills` still loads
  (folded into one source) and is rewritten to `[[source]]` on save.
- **Any git repo can expose skills**: discovery is a recursive scan — any directory
  with a valid `SKILL.md` (kebab name == dir, description, `metadata.axis`) is a
  skill. A per-source `root` scopes the scan. No `skills/` layout required.
- **Selectors are pattern or path**: each source's `skills` entries are axis/glob/
  name (matched against that source's discovered names) or `path:<repo-rel-dir>`.
- **Whole-directory vendoring**: a skill's entire directory (SKILL.md + supporting
  files) is materialized via an atomic temp+rename dir swap; the lock hash is a
  deterministic tree hash; drift = the on-disk tree hash differs.
- **Collisions fail loud**: two sources resolving the same skill name errors and
  names both sources.
- **Lock pins origin**: `.praktik/lock.json` records each skill's `source/repo/ref/
  commit/hash` and a `sources` list, for reproducible resync.
- **Commands**: `source add/rm/ls` manage the source set; `add/rm` take `--from
  <alias>` to choose which source; `status`/`diff`/`doctor` are source-aware.

The single-source narrative below remains accurate for the reconcile invariants
(real copies, prune-only-managed, atomic writes, idempotent sync) — read it with
"the library" generalized to "each resolved source".

## Conceptual data model

- **Library** — a praktik skill repo's `skills/` dir (source of truth). Resolved from `$PRAKTIK_DIR/skills`,
  global config, or a git remote cloned into `~/.cache/praktik/<repo>@<ref>`.
- **Skill** — `skills/<name>/SKILL.md`: YAML frontmatter (`name` kebab == dirname, `description` 10–1024
  chars, `metadata.axis`) + markdown body. Content-addressed by a SHA256 of the file.
- **Axis** — prefix grouping and the batch unit: `index, proc, flow, tool, lang-*, build-*, arch, data,
  framework, ci`. `lang-java` expands to `lang-java-*`; bare names and globs also match.
- **Target** — an agent's skills dir: `claude → .claude/skills`, `agents → .agents/skills`
  (vendor-neutral, read by Copilot/OpenHands), `codex → .codex/skills`, `github → .github/skills`,
  `opencode → .config/opencode/skills`, `pi → .pi/agent/skills`. Project-relative or `$HOME` (global).
- **Vendoring** — materialize the desired skill set into every target as **real copies** (no symlinks).

## On-disk state

Three files, replacing `.skillshare/config.yaml` + `.skillshare/skills/` + the SHA manifest.

**`praktik.toml`** (project root, committed — the human-edited desired state):
```toml
[source]
repo = "github.com/irasikhin/praktik-skills"   # or a local path; default: $PRAKTIK_DIR / global config
ref  = "main"                          # branch or tag; pin a project by tag

[targets]
claude = ".claude/skills"              # add agents/codex/github/opencode/pi as needed

skills = ["proc", "flow", "lang-java", "build-maven", "praktik-index"]   # axes | globs | exact names
```

**`.praktik/lock.json`** (project, committed — machine-written, reproducible resync + drift detection):
```json
{ "source": { "repo": "...", "ref": "main", "commit": "0a1b2c3" },
  "skills": { "lang-java-naming": { "hash": "96b9..." }, "proc-conventional-commits": { "hash": "..." } } }
```
The lock's `skills` map **is the managed set** — `sync` only ever prunes files it put there, never foreign
content. `commit` pins exactly what was resolved so another machine reproduces byte-for-byte.

**`~/.config/praktik/config.toml`** (global — defaults for `-g` vendoring into `$HOME` and the default
library/targets), mirroring kpass's `~/.config/kpass/config.toml` precedence (flags > env `PRAKTIK_*` >
config > defaults). Global mode tracks its managed set in `~/.local/state/praktik/lock.json`.

## Command surface (the API / UX)

Kong root struct (`praktikCLI`) with global flags `--json`, `--no-color`/`-C`, `--yes`/`-y`,
`-g/--global`, `--source <path|repo>`, `--ref <ref>`, `-V/--version` — the kpass `commands.go` pattern.
Each command is one file in `internal/cli/`, a `Run(c *ctx) error` method; `ctx` carries I/O streams,
resolved config, the library handle, and the project handle (lazily opened like kpass `openDatabase()`).

**Consumption** (project by default, `-g` for global):
| Command | Purpose |
|---|---|
| `praktik init [<axis\|name>...]` | Scaffold `praktik.toml` (+ optional starter skills) and sync. |
| `praktik add <axis\|glob\|name>...` | Add to `skills=` in the manifest, then `sync`. |
| `praktik rm <axis\|glob\|name>...` | Remove from the manifest, then `sync` (prunes from targets). |
| `praktik sync` | **The workhorse.** Reconcile every target to the manifest (add/update/prune); idempotent. |
| `praktik update [<axis\|glob>...]` | Re-resolve from source (pull / bump `ref`), rewrite lock, sync. Replaces the broken skillshare update. |
| `praktik status` | Drift view: `+added ~changed -pruned =ok` across manifest vs lock vs targets vs source. `--json`. |
| `praktik diff [<name>]` | Per-skill diff between a vendored copy and source (the detail behind `~changed`). |

**Discovery / polish** (the "convenient UI/UX" ask — port kpass `tree`/`picker`/`color`):
| Command | Purpose |
|---|---|
| `praktik list` | Available skills in the library, grouped by axis, with one-line summaries. `--installed`, `--json`. |
| `praktik search <query>` | Fuzzy match over name/description/axis. `--json`. |
| `praktik show <name>` | Pretty-render a skill's frontmatter + body. `--json`. |
| `praktik pick` | Interactive fzf picker over the library → action (add/show/rm). Marquee UX, kpass `pick` pattern. |

**Authoring** (replaces the python scripts, runs against the library repo):
| Command | Purpose |
|---|---|
| `praktik new <name>` | Scaffold `skills/<name>/SKILL.md` with a frontmatter template (axis/description via flags or prompt). |
| `praktik validate` | Lint all SKILL.md frontmatter. Hard gate (exit 1). `--json`. Ports `validate-skills.py`. |
| `praktik handbook [--check]` | Regenerate `HANDBOOK.md`; `--check` = CI staleness gate. Ports `gen-handbook.py`. |
| `praktik doctor` | Sanity check: source resolves, targets writable, lock in sync, leftover `.skillshare/` detected. |

**Plumbing:**
| Command | Purpose |
|---|---|
| `praktik target add <agent> <path>` / `rm` / `ls` | Manage the target set in `praktik.toml`. |
| `praktik migrate` | One-shot import of an existing `.skillshare/config.yaml` → `praktik.toml` + sync (see below). |
| `praktik completion [bash\|zsh\|fish]` | Shell completions (generated at install). |

## The reconcile engine (core contract)

`internal/sync` is the heart; `add`/`rm`/`update`/`init` all funnel into it. Given (desired set from
manifest, resolved library at `ref`, targets, current lock):

1. **Resolve source** — ensure the library is present at `ref` (`$PRAKTIK_DIR`, local path, or clone/pull
   into `~/.cache/praktik`); capture its commit. Never records a temp path — identity is `(repo, ref, name)`.
2. **Expand** desired axes/globs/names → concrete skill names (the bash `match()` logic, in Go).
3. **Plan** per target by comparing source hashes against the lock: `install` (missing), `update`
   (source hash ≠ lock hash), `prune` (in lock but no longer desired), `noop` (desired & unchanged).
4. **Apply** atomically per skill: write to a temp file, `rename` into place (kills the `chmod` dance and
   partial-sync inconsistency).
5. **Write lock** — new hash per managed skill + the source commit.

This single contract fixes every documented pain point: dead temp paths (identity by name+ref),
non-atomic sync (temp+rename), no drift (hashes + `status`/`diff`), unsafe pruning (managed-set only),
chmod hacks (we own the files), no pinning (`ref` in manifest, `commit` in lock).

## Internal architecture (Go packages — kpass three-layer)

```
cmd/praktik/main.go        # thin: inject version, call cli.Run(); `exit` seam for tests (kpass main.go)
internal/cli/             # kong command tree, one file per command, ctx injection, UserError type
  commands.go ctx.go add.go rm.go sync.go status.go list.go show.go pick.go new.go validate.go handbook.go ...
internal/library/         # resolve repo (local/$PRAKTIK_DIR/git cache), read skills, expand axis/glob/name
internal/skill/           # Skill type: frontmatter parse + validate, body, content hash
internal/project/         # praktik.toml + lock.json load/write; desired-set + managed-set
internal/sync/            # the reconcile engine (plan + apply); atomic temp+rename writer
internal/handbook/        # gen + --check (port gen-handbook.py: GROUP_ORDER, summarize, render)
internal/validate/        # frontmatter lint (port validate-skills.py: KEBAB, desc 10..1024, name==dir)
internal/config/          # global ~/.config/praktik/config.toml, env PRAKTIK_*, precedence resolver
internal/color/  tree/  picker/   # UX — copy kpass's self-contained packages near-verbatim
```

Reuse, don't reinvent: copy `internal/color`, `internal/tree`, `internal/picker` and the `ctx`/kong
wiring from kpass's `internal/`. The frontmatter parser must match the python field reader exactly
(`split_frontmatter` + folded/literal block handling in `validate-skills.py:25-56` /
`gen-handbook.py:43-72`) so existing skills validate identically.

## Migration from skillshare

`praktik migrate` reads `.skillshare/config.yaml`, takes the skill **names** (ignoring the dead `source:`
temp paths), writes them as `skills=` in a new `praktik.toml`, reads `targets:` into `[targets]`, runs
`sync` (real copies), then removes the old `.skillshare/` dir, the per-target `.skillshare-manifest.json`,
and the `.claude/skills/*` symlinks it replaced. Lets every already-vendored repo cut over in one
command. `doctor` flags a leftover `.skillshare/` (suggesting `migrate`)
and any leftover `.skillshare-manifest.json`.

## Managed praktik:skills doc block (added post-design, v0.3.0)

Per `proc-skill-single-source`, a repo's agent doc (CLAUDE.md / AGENTS.md) lists which skills are active —
hand-maintained, it drifts from `praktik.toml`. So the doc opts in with a `<!-- praktik:skills BEGIN -->` /
`<!-- praktik:skills END -->` marker pair (a pre-existing convention in vendored repos), and every
reconcile (everything funnelling through `runSync`: init/add/rm/sync/update/source/target/migrate)
rewrites the lines between the first marker pair to the lock's managed set — a sorted, comma-separated
list wrapped at 80 columns. `status`/`doctor` report a stale block (drift = sync needed); a lone
BEGIN/END marker is a hard error (a silently unmanaged block would just drift); no markers → no writes.
Implementation: `internal/project/docsblock.go` (`SyncDocs` / `DocsStatus`).

## Build / packaging / distribution (match kpass + sandboxer)

- Go 1.26, module `github.com/irasikhin/praktik` (or chosen path); `buildGoModule` in `nix/package.nix`
  with `ldflags -X main.version=v${version}`, `wrapProgram` to put `git`/`fzf` on PATH, generated
  bash/zsh/fish completions in `postInstall` (kpass `nix/package.nix`).
- Flake (`flake.nix`) with overlay + devShell + `nix flake check`.
- Release on `v*` tag via `.github/workflows/release.yml`: multi-arch
  (linux amd64/arm64, darwin), tag-matches-package.nix check, tarball + SHA256 (kpass/sandboxer pattern).
- Version: ldflags primary, `debug.ReadBuildInfo()` VCS fallback (kpass `version.go`).

The existing praktik **skill** repo changes: its CI `validate.yml` calls `praktik validate` and
`praktik handbook --check` instead of the python scripts; `scripts/git-hooks/pre-commit` calls
`praktik handbook`. The python scripts and the `praxis-skills` bash wrapper are deleted once the binary is
published. `.claude-plugin/` (marketplace fallback) is unaffected.

## Phased implementation

1. **Scaffold** the new Go repo: `cmd/praktik`, kong root, `ctx`, `--version`, vendored `color`/`tree`/
   `picker` from kpass; `flake.nix` + `nix/package.nix` building a `dev` binary.
2. **Library + skill layer**: resolve `$PRAKTIK_DIR`/local path, read + parse + hash skills, axis/glob/name
   expansion. Port `validate` (golden-tested against the full library → identical pass) and
   `handbook` (`--check` must match the committed `HANDBOOK.md` byte-for-byte).
3. **Project state**: `praktik.toml` + `lock.json` read/write; `init`, `target`.
4. **Reconcile engine** + `add`/`rm`/`sync`/`update`/`status`/`diff`. Atomic writer. Project scope first.
5. **Global scope** (`-g`) + `~/.config/praktik/config.toml`; git clone-on-demand source with `ref` pinning.
6. **UX commands**: `list`, `search`, `show`, `pick`, `doctor`, `completion`.
7. **`migrate`**; then cut the praktik skill repo's CI/hooks over to the binary and remove the bash + python
   tooling.

## Verification (end-to-end)

- **Authoring parity (no regressions):** `praktik validate` against the library checkout exits 0 on
  every skill and reproduces failures the python script reports (mutate a name/description in a temp copy);
  `praktik handbook --check` passes against the committed `HANDBOOK.md` and `praktik handbook` reproduces it
  byte-for-byte (`git diff --exit-code HANDBOOK.md`).
- **Vendor lifecycle in a scratch repo:** `praktik init lang-java proc` → assert real files (not symlinks)
  under `.claude/skills/`, `praktik.toml` + `.praktik/lock.json` written, no temp paths anywhere; `praktik
  add build-maven` then `praktik status` shows `+`; `praktik rm proc` prunes only proc-* and leaves others;
  re-running `praktik sync` is a clean noop (`status` all `=ok`).
- **Drift + update:** edit a vendored copy → `status` shows `~changed`, `diff` shows it; `praktik sync`
  restores from source; bump a skill in the source → `praktik update` rewrites the lock hash + commit.
- **Migration:** run `praktik migrate` in a skillshare-vendored repo (dead temp source paths) → `praktik.toml` lists the same skill names, real copies replace the symlinks, `.skillshare/`
  removed, `status` all `=ok`.
- **Tests:** table-driven unit tests with `ctx` I/O injection and a `picker.Hook` seam (kpass pattern);
  `go test ./...` + `nix flake check` green; `praktik doctor` clean on a migrated repo.

## Landscape watch (2026-08)

The niche praktik occupies — a committed manifest + lock with enforced restore and a local
drift diff — is explicitly unclaimed by the Agent Skills spec
([agentskills#243](https://github.com/agentskills/agentskills/discussions/243),
[#210](https://github.com/agentskills/agentskills/discussions/210)). Events that would
collapse it; re-check when returning to this repo:

- vercel-labs/skills ships restore-from-lock
  ([#283](https://github.com/vercel-labs/skills/issues/283),
  [#549](https://github.com/vercel-labs/skills/issues/549)) and/or a declarative manifest
  ([#141](https://github.com/vercel-labs/skills/issues/141)).
- `gh skill` (GitHub CLI ≥ 2.90) gains a set-level manifest + sync — today it pins and
  drift-checks single skills only.
- agentskills#243 (`sklz.json`) lands in the spec — then align `praktik.toml`/lock with the
  standard instead of competing with it.

Deliberately out of scope (commoditized elsewhere): many-agent fan-out (vercel skills: 75+
targets), registry/discovery, Nix-native declarative skill install (agent-skills-nix,
nix-skills).
