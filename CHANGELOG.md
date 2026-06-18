# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial release: a standalone CLI for the OpusClip API.
- `clip create` — submit a video URL for clipping, with curation flags
  (`--genre`, `--lang`, `--keywords`, `--duration`, `--start`/`--end`,
  `--brand-template`), conclusion actions (`--webhook`, `--email`), and `--wait`.
- `clip get` / `clip watch` — fetch a project, or poll it through its stages with
  a live stage stepper (`--interval`, `--timeout`, `--exit-status`).
- `clips list` / `clips download` — list a project's generated clips and download
  the rendered mp4s (`--out`, `--clip`, `--dry-run`, JSON manifest).
- `auth login/logout/status/switch/token` — API-key auth with OS-keychain storage
  and a file fallback; `--org` for multi-org users (`x-opus-org-id`).
- `api` — raw authenticated request escape hatch, with bare-array `--paginate`.
- `config`, `profiles`, `doctor`, `info`, `completion`, `docs`, `update`, `version`.
- Output adapts to context (table on a TTY, JSON when piped) with `--json`,
  `--jq`, `--columns`, and `-o table|json|csv|tsv|yaml`.
- Stable exit codes (`3` auth, `4` forbidden, `5` not-found, `6` rate-limit,
  `7` validation, `8` upstream/STALLED, `124` timeout).
- Claude Code skill + plugin for agent-driven clipping.
