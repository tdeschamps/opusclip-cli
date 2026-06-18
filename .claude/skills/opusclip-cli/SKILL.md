---
name: opusclip-cli
description: Use when clipping videos with the `opusclip` CLI — submit a video URL, watch a project render through its stages, and list/download the generated clips. Covers the create→watch→download flow, the stage stepper, --wait/--exit-status semantics, and --json/--jq idioms so commands work first try.
---

# Driving the `opusclip` CLI efficiently

`opusclip` wraps the OpusClip REST API (`https://api.opus.pro`). Output is a human
table on a TTY and **JSON when piped**, so in scripts/agents you almost always
want `--json`.

## The core flow

1. **Create** a project from a video URL → returns a `projectId`.
2. **Watch** (or `create --wait`) until the project reaches `COMPLETE` (or fails
   at `STALLED`). Project stages are `PENDING → QUEUED → IMPORT → CURATE → REFINE
   → RENDER → UPLOAD → COMPLETE`; there is no percentage, so progress is a stepper.
3. **List / download** the generated clips (the export URLs are pre-signed).

```sh
pid=$(opusclip clip create --url "https://youtu.be/XXXX" --json --jq '.[0].projectId')
opusclip clip watch "$pid"
opusclip clips download --project "$pid" --out ./clips
```

Or in one shot (blocks until done):

```sh
opusclip clip create --url "https://youtu.be/XXXX" --wait
```

## Auth (do this once)

Token resolves: `--api-key`/`--token` flag → `OPUSCLIP_API_KEY`/`OPUSCLIP_TOKEN`
env → stored credential. Quickest for scripting:

```sh
export OPUSCLIP_API_KEY=sk_xxx      # from https://clip.opus.pro/dashboard
opusclip doctor                     # verify connectivity + credentials
```

Multi-org users: pass `--org <id>` or set `OPUSCLIP_ORG_ID` (sent as
`x-opus-org-id`). If credentials were saved with `OPUSCLIP_NO_KEYRING=1`, keep
that env var set so later commands read the same (file) store.

## Commands and their real flags

| Command | Key flags |
| --- | --- |
| `clip create` | `--url` (required), `--wait`, `--interval`, `--timeout`, `--exit-status`, `--webhook`, `--email`, `--notify-failure`, `--genre`, `--lang`, `--keywords`, `--duration min:max` (repeatable), `--start`/`--end`, `--brand-template` |
| `clip get <projectId>...` | (one or more ids) |
| `clip watch <projectId>` | `--interval` (default 6s), `--timeout` (default 30m), `--exit-status` |
| `clips list` | `--project` (required), `--limit`, `--all` |
| `clips download` | `--project` (required), `--out <dir>`, `--clip <id>`, `--dry-run` |
| `api <METHOD> <path>` | `--param k=v` (repeatable), `--field k=v` (body), `--paginate` (bare-array lists), `--input -` |

**Global flags worth knowing:** `--json` (force JSON), `--jq '<filter>'` (built-in
gojq, no external jq), `--limit N`, `--all`, `--columns a,b`, `--org <id>`,
`--profile <name>`, `-o table|json|csv|tsv|yaml`.

## Recipes (copy these patterns)

Create and capture the project id:
```sh
pid=$(opusclip clip create --url "$VIDEO" --json --jq '.[0].projectId')
```

Watch with a tight loop in CI and branch on the outcome (exit 0 = COMPLETE,
8 = STALLED, 124 = timeout):
```sh
opusclip clip watch "$pid" --interval 6s --timeout 20m --exit-status || echo "render failed ($?)"
```

List clips, pull just the export URLs:
```sh
opusclip clips list --project "$pid" --json --jq '.[].uriForExport'
```

Download a single clip by its id:
```sh
opusclip clips download --project "$pid" --clip "$pid.c1" --out ./clips
```

Raw escape hatch for anything the typed commands don't cover:
```sh
opusclip api GET /api/clip-projects/"$pid"
opusclip api GET /api/exportable-clips --param q=findByProjectId --param projectId="$pid" --paginate
```

## Pitfalls

- **`clip create` only takes a `--url`** (a public/streamable video URL). Local
  file upload is not supported in v0.1.
- **There is no `clip list` (list-all-projects)** — the public API has no such
  endpoint. Track project ids you create. (`opusclip api GET /api/clip-projects
  --paginate` may work if an undocumented list exists.)
- **POST `clip create` is not auto-retried on 429** (it's non-idempotent); a rate
  limit surfaces as exit 6. GET-based polling/watch *does* honor `Retry-After`.
- **Watch interval:** the API budget is ~30 req/min. The default 6s is safe; don't
  poll faster than ~2s in long loops.
- **Virality/judge scores** are best-effort — they appear in `clips list` only
  when the live API returns them (they're not in the public spec).
- Exit codes are stable: `3` auth, `4` forbidden (incl. monthly cap), `5`
  not-found, `6` rate limited, `7` validation, `8` upstream/STALLED, `124`
  timeout — branch on these.

## Targeting an environment

Endpoints resolve flag → env → profile → default. Override with
`OPUSCLIP_BASE_URL` or `opusclip --profile <name> …`. `opusclip info` shows what's
currently resolved.
