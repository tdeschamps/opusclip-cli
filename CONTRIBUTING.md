# Contributing to the OpusClip CLI

Thanks for contributing! The CLI is built **test-first** and aims to be easy to
hack on — no native toolchain beyond Go.

## Quickstart (3 commands)

```sh
git clone https://github.com/tdeschamps/opusclip-cli
cd opusclip-cli
make test      # runs the full suite
```

Then build a local binary:

```sh
make build     # ./bin/opusclip
```

## Development loop: Red → Green → Refactor

No production code is written without a failing test that demanded it. When you
open a PR, the template asks **"which test drove this change?"**

- **Unit tests** (the bulk): pure functions — flag parsing, the create-input
  builder, the stage-stepper poller, config precedence, retry/backoff,
  renderers, `--jq`, exit-code mapping.
- **Integration tests**: the api client and commands against `httptest.Server`.
  Assert request shaping and response decoding.
- **End-to-end**: `testscript` `.txtar` files in `test/script` drive the real
  binary (stdin in, stdout/stderr/exit-code asserted).

## Before you push

```sh
make fmt       # gofumpt + goimports
make lint      # golangci-lint
make test      # go test -race ./...
```

## Conventions

- `gofumpt` + `golangci-lint` keep review about behavior, not style.
- Conventional-commit messages drive the changelog.
- JSON output is treated as an API — changes to `-o json` shapes need care and
  are locked down by tests.
- See `ARCHITECTURE.md` for where code goes and the "add a resource command"
  recipe.
