# Architecture

This document explains how `opusclip` is structured so you know where new code
goes.

## Layers

Dependencies point **downward only**. The `cmd` layer never builds HTTP requests
directly — it calls the `api` adapter behind a method seam, and those methods are
the test seams.

```
 cmd layer (Cobra commands)        thin: parse flags, call adapters, render
 ─────────────────────────────────────────────────────────────────────────
 api (OpusClip REST) │ output (renderers)                  adapters
 ─────────────────────────────────────────────────────────────────────────
 platform: config, auth/keychain, iostreams, httpclient (RoundTripper chain)
```

## Package map

| Package | Responsibility |
|---|---|
| `cmd/opusclip` | Entrypoint: build root command, execute, map errors → exit codes. Hosts the `testscript` e2e harness. |
| `internal/cmd/*` | One package per command group. Thin; depends on `cmdutil` + adapters. |
| `internal/cmdutil` | The `Factory` (dependency injection), global flags, exit-code mapping, render/prompt/browser helpers. |
| `internal/api` | OpusClip REST adapter: typed models (`Project`, `Stage`, `ExportableClip`), the thin `Client` (create/get/list/validate), and the `Raw` escape hatch. |
| `internal/config` | TOML config, profiles, precedence resolution (flag → env → profile → default). |
| `internal/auth` | Credential modeling and storage (OS keychain with a 0600 file fallback). |
| `internal/output` | Renderers (table/json/csv/tsv/yaml) and the built-in `--jq` filter. |
| `internal/iostreams` | Testable stdin/stdout/stderr with TTY/color detection; spinner used by the stage stepper. |
| `internal/httpclient` | The `RoundTripper` chain: auth → retry (429/`Retry-After`, idempotent only) → logging. |
| `internal/updatecheck` | Background "new version available" check. |

## The async (render) model

OpusClip projects are long-running and report a coarse **stage** enum
(`PENDING → QUEUED → IMPORT → CURATE → REFINE → RENDER → UPLOAD → COMPLETE`, with
`STALLED` as the failure state) — there is **no percentage**. So progress is a
**stage stepper**, not a progress bar.

`internal/cmd/clip/poll.go` holds the shared poller used by `clip watch` and
`clip create --wait`:

- Polls `GetProject` on an interval (default 6s, under the 30 req/min budget).
- Renders the stepper on stderr via the iostreams spinner; on a non-interactive
  stderr it emits one terse line per stage change.
- Returns `nil` on `COMPLETE`, a `SilentError(ExitUpstream)` on `STALLED`, and
  the context error on cancel/deadline (a deadline maps to exit `124`).
- The tick is injected via the package-level `after` seam and the `Factory.Clock`
  so tests run instantly and deterministically.

## Key seams (interfaces)

- HTTP goes through an injectable base `http.RoundTripper` so tests swap in a fake.
- The poller depends on a small `projectGetter` interface (`GetProject`) so tests
  script stage sequences without a server.
- `auth.Store` abstracts credential persistence (`MemoryStore` in tests).
- `output.Printer` writes to an `io.Writer` so tests assert on rendered bytes.
- Downloads use a **separate bare `http.Client`** (no bearer token) because the
  export URLs are pre-signed GCS links.

## Adding a command (recipe)

1. Add the model + client method in `internal/api` (with an httptest test).
2. Create or extend `internal/cmd/<group>` exposing `NewCmd<Group>(f *cmdutil.Factory)`.
3. Define the `output.Field` list and call `cmdutil.RenderSlice` / `GetAndRender`.
4. Register it in `internal/cmd/root`.
5. Add a `.txtar` script under `test/script` for the user-facing contract.
