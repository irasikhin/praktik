# Changelog

All notable changes to this project are documented here. Format based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

`scripts/release.sh` appends new sections automatically from
[Conventional Commits](https://www.conventionalcommits.org/) — please write
commit messages with that in mind. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## [0.1.2] — 2026-08-10

### Docs

- README repositioned against the unclaimed skill-distribution gap
  (agentskills#243, vercel-labs/skills#283); vendor-neutral `.agents/skills` /
  `.github/skills` target examples; a landscape-watch section in
  `docs/design.md`.

### Chores

- Republished with a fresh history after a privacy scrub. The module proxy's
  `v0.1.0` / `v0.1.1` entries are stale — install `@v0.1.2` (or `@latest`).

## [0.1.1] — 2026-08-10

### Docs

- design notes scrubbed and republished. The Go module proxy pins `v0.1.0`
  to a superseded pre-publish revision, so `go install …@v0.1.0` fails its
  checksum — install `@v0.1.1` (or `@latest`) instead.

## [0.1.0] — 2026-08-10

Initial public release.

### Added

- `praktik` — a self-contained CLI for vendoring agent skills (`SKILL.md`
  directories) from any git repo or local path into agent targets, Maven-style:
  `praktik.toml` `[[source]]` entries + a committed `.praktik/lock.json`,
  reconciled as real directories (no symlinks) by `init` / `add` / `rm` /
  `sync` / `update` / `status` / `diff` / `source` / `target`.
- Discovery & authoring: `list`, `search`, `show`, `pick` (fzf), `new`,
  `doctor`, `validate`, `handbook`, shell `completion`.
- Tool-managed `<!-- praktik:skills BEGIN/END -->` block in `CLAUDE.md` /
  `AGENTS.md`, rewritten on every reconcile.
- Migration paths: `praktik migrate` imports a `.skillshare/` setup, renames
  legacy praxis-named state (`praxis.toml`, `.praxis/`, old global
  config/state), and upgrades legacy `praxis:skills` doc markers, which stay
  readable until migrated.

### Changed

- Republished at `github.com/irasikhin/praktik` with a fresh history.
