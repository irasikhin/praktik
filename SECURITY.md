# Security Policy

## Supported Versions

praktik is pre-1.0; the CLI flags, manifest schema, and on-disk layout may
still change between minor versions. Security fixes are provided for the
latest release only.

## Reporting a Vulnerability

Do not open a public issue for a vulnerability. Report security issues by
emailing the maintainer listed in the GitHub repository profile, or by using
GitHub private vulnerability reporting if it is enabled for this repository.

Please include:

- affected praktik version or commit;
- operating system and install method;
- a minimal reproduction.

## Scope Notes

praktik shells out to `git` to clone skill sources declared in
`praktik.toml`, and vendors files from those checkouts into agent target
directories. Treat skill sources like any other dependency: only add
repositories you trust, and review what a `sync` vendors into your tree.
