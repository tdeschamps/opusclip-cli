<div align="center">

# opusclip

**A standalone CLI for the OpusClip API — submit a video, watch it render, download the clips. Scriptable, agent-friendly, and at home in any terminal.**

[![CI](https://github.com/tdeschamps/opusclip-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/tdeschamps/opusclip-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tdeschamps/opusclip-cli.svg)](https://pkg.go.dev/github.com/tdeschamps/opusclip-cli)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Coverage](https://img.shields.io/badge/coverage-90%25-brightgreen)](#development)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-fe5196.svg)](https://www.conventionalcommits.org)

<sub>Go Report Card, codecov, and the release/version badges activate once the repository is public.</sub>

</div>

---

`opusclip` wraps the **[OpusClip API](https://help.opus.pro/api-reference/overview)**
(`https://api.opus.pro`) behind one consistent interface, delivered as a single
static binary. It turns long-form videos into short clips from the command line:

- **Creators / editors** — clip a video URL and pull the results without opening the app.
- **Developers / integrators** — script the create→render→download pipeline, pipe JSON, wire CI jobs.
- **AI / agent power users** — drive clipping non-interactively with stable `--json` output and exit codes.

Design north star: feel like `gh`, `stripe`, and `sentry-cli` — predictable
noun-verb commands, great `--help`, machine-readable output in pipes,
human-readable in a TTY, first-class auth, and a stage-stepper for long-running
renders.

## Table of contents

- [Install](#install)
- [Quickstart](#quickstart)
- [Commands](#commands)
- [Output & scripting](#output--scripting)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Exit codes](#exit-codes)
- [Known gaps & roadmap](#known-gaps--roadmap)
- [Development](#development)
- [License](#license)

## Install

```sh
# Homebrew (macOS/Linux)
brew install tdeschamps/tap/opusclip

# Shell installer (downloads the latest signed release binary)
curl -fsSL https://raw.githubusercontent.com/tdeschamps/opusclip-cli/main/install.sh | sh

# From source (Go 1.25+)
go install github.com/tdeschamps/opusclip-cli/cmd/opusclip@latest
```

Prebuilt, signed binaries for `darwin/{amd64,arm64}`, `linux/{amd64,arm64}`, and
`windows/amd64` are attached to every [release](https://github.com/tdeschamps/opusclip-cli/releases).

> API access requires a qualifying OpusClip plan (Pro/Max/Enterprise, currently
> in beta). Get a key from the [dashboard](https://clip.opus.pro/dashboard).

## Quickstart

```sh
# 1. Authenticate (paste a key interactively, or pipe it in CI)
opusclip auth login                              # paste a key from clip.opus.pro/dashboard
echo "$OPUSCLIP_API_KEY" | opusclip auth login --with-token
opusclip auth status

# 2. Clip a video and block until the clips are ready
opusclip clip create --url "https://youtu.be/LXvv6CbGg8A" --wait

# 3. …or do it in steps and capture the project id
pid=$(opusclip clip create --url "https://youtu.be/LXvv6CbGg8A" --json --jq '.[0].projectId')
opusclip clip watch "$pid"                       # live stage stepper to COMPLETE

# 4. List and download the generated clips
opusclip clips list --project "$pid"
opusclip clips download --project "$pid" --out ./clips

# 5. Raw escape hatch — anything the typed commands don't cover yet
opusclip api GET /api/clip-projects/"$pid"
```

## Commands

```
opusclip
├── auth        login | logout | status | switch | token
├── config      get | set | list | edit
├── profiles    list | use
├── clip        create | get | watch
├── clips       list | download
├── api         raw authenticated request escape hatch
├── info        version, configuration & status (logo banner)
├── doctor      connectivity & credential diagnostics
├── completion  bash | zsh | fish | powershell
├── docs        open the docs
├── update      self-update
└── version
```

`clip create` accepts a video `--url` plus optional curation flags (`--genre`,
`--lang`, `--keywords`, `--duration min:max`, `--start`/`--end`,
`--brand-template`) and conclusion actions (`--webhook`, `--email`). Add `--wait`
to block until the render finishes.

## Output & scripting

- **TTY** → colorized, aligned tables. **Piped/redirected** → JSON.
- Override anytime with `-o table|json|csv|tsv|yaml` or the `--json` shorthand.
- Built-in **`--jq`** filter (powered by [gojq](https://github.com/itchyny/gojq);
  no external `jq` needed): `opusclip clips list --project P1 --json --jq '.[].uriForExport'`.
- `--columns id,title` to pick/order columns for tables and CSV.
- Long-running renders show a **stage stepper** on stderr; `--wait`/`watch`
  return a stable [exit code](#exit-codes) by outcome (`0` complete, `8`
  stalled, `124` timeout).

```console
$ opusclip clips list --project P1
ID      TITLE          DURATION  GENRE    SCORE  CREATED
P1.c1   Best Moment    00:30     podcast  87     2026-06-17T10:05:00Z

$ opusclip clips list --project P1 --json --jq '.[].uriForExport'
https://storage.googleapis.com/...
```

## Authentication

`opusclip` resolves the key in this order: `--api-key`/`--token` flag →
`OPUSCLIP_API_KEY`/`OPUSCLIP_TOKEN` env → stored credential for the active
profile. Secrets are stored in the **OS keychain** when available (macOS
Keychain, Windows Credential Manager, libsecret/kwallet), with a `0600` file
fallback. Set **`OPUSCLIP_NO_KEYRING`** to skip the keychain and persist to the
`0600` file — handy in CI, headless shells, or on macOS where the keychain can
pop an interactive prompt.

Keys are never printed back — `opusclip auth status` shows only a masked
fingerprint. OpusClip authenticates with an API key only (no OAuth). Multi-org
users can pass `--org <id>` or set `OPUSCLIP_ORG_ID` (sent as `x-opus-org-id`).

## Configuration

Config lives at `~/.config/opusclip/config.toml` (XDG-respecting;
`%APPDATA%\opusclip\config.toml` on Windows). Any setting resolves
**flag → env var → profile → built-in default**.

```toml
active_profile = "default"

[profiles.default]
base_url      = "https://api.opus.pro"
org_id        = ""
output        = "table"
default_limit = 50
```

Manage it with `opusclip config get/set/list/edit` and `opusclip profiles
list/use`. Relevant env vars: `OPUSCLIP_API_KEY`, `OPUSCLIP_TOKEN`,
`OPUSCLIP_PROFILE`, `OPUSCLIP_BASE_URL`, `OPUSCLIP_ORG_ID`, `OPUSCLIP_OUTPUT`,
`OPUSCLIP_NO_KEYRING`, `OPUSCLIP_NO_UPDATE_NOTIFIER`.

## Claude Code / agents

A Claude Code **skill** ships with this repo so agents drive the CLI efficiently
(the create→watch→download flow, `--wait`/`--exit-status` semantics, `--json`/`--jq`
idioms). It loads automatically inside this checkout. To get it in your own
projects:

```sh
/plugin marketplace add tdeschamps/opusclip-cli
/plugin install opusclip-cli@opusclip
```

## Exit codes

| Code | Meaning              | Code  | Meaning                          |
| ---- | -------------------- | ----- | -------------------------------- |
| `0`  | success              | `5`   | not found (404)                  |
| `1`  | generic error        | `6`   | rate limited (429)               |
| `2`  | usage / bad flags    | `7`   | validation (422)                 |
| `3`  | auth required (401)  | `8`   | upstream / server / STALLED      |
| `4`  | forbidden (403, incl. monthly cap) | `124` | operation timed out |

## Known gaps & roadmap

v0.1 deliberately ships the core clip workflow. The following are **not** in v0.1
(documented here so there are no surprises):

- **No `clip list` (list-all-projects)** — the public API exposes no project-list
  endpoint. Track the `projectId`s you create. *(v0.2 if an endpoint appears.)*
- **No local file upload** — `clip create` takes a `--url` only; the upload
  endpoint isn't in the public spec. *(v0.2.)*
- **Virality/judge scores** are best-effort — surfaced only when the live API
  returns them (they're absent from the public spec).
- **Webhook payloads** are opaque — `--webhook` registers the URL; the delivered
  payload contract isn't modeled.
- **Out of scope (v0.1):** collections, brand-template management, social
  posting, censor jobs, AI thumbnails, transcripts.

## Development

Built **test-first** in Go behind interface seams (see
[`ARCHITECTURE.md`](ARCHITECTURE.md) and [`CONTRIBUTING.md`](CONTRIBUTING.md)).

```sh
make test       # go test -race ./...
make cover      # coverage profile + total (fails under the 90% gate)
make lint       # golangci-lint
make fmt        # gofumpt + goimports
make build      # ./bin/opusclip
```

The CI gate runs `gofumpt`, `golangci-lint`, `go vet`, `govulncheck`, the full
race-enabled test suite (unit + `testscript` e2e), and a cross-compile matrix.

## License

[MIT](LICENSE) © Thomas Deschamps
