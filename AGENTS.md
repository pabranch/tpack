# tpack Agent Guide

## Core Commands

- **Build**: `make build` (outputs to `dist/tpack`).
- **Lint**: `make lint` / `make lint-fix` (requires `golangci-lint`).
- **Test**:
  - `make test` or `make test-unit`: Core logic in `internal/`.
  - `make test-integration`: Integration tests in `tests/integration/`.
  - `make test-e2e`: End-to-end tests in `tests/e2e/`.
  - `make test-all`: Runs all of the above.
- **Docs**: `make docs-serve` (requires `pip install -r requirements.txt`).
- **Coverage**: `make coverage` (unit tests only, outputs to `coverage.out`).

## Architecture & Entrypoints

### Go Binary (`cmd/tpack/`)
The `main.go` delegates to `Execute()` which sets up a Cobra CLI:
- `tpack init` — Default command when running `tpack` with no args. Checks tmux version, binds keys, sources plugins, optionally spawns background self/update checks. This is what the `tpm` shim calls.
- `tpack install` — Installs all plugins declared in tmux.conf.
- `tpack update [plugin...]` — Updates plugins (or "all").
- `tpack clean` — Removes plugins not in tmux.conf.
- `tpack tui [--install|--update|--clean]` — Opens the interactive TUI.
- `tpack source` — Sources plugin `.tmux` files without installing.
- `tpack check-updates` — Background update checker.
- `tpack self-update` — Background tpack binary updater.
- `tpack commits [plugin]` — Shows commit history for a plugin.
- `tpack completion` — Shell completion setup.

### Shell Scripts
- **`tpm`** — Root-level shell shim for TPM compatibility. Delegates to `tpack init` via `lib/find_binary.sh`.
- **`bindings/`** — Shell scripts invoked by tmux keybindings (`install_plugins`, `update_plugins`, `clean_plugins`). All delegate to `tpack install/update/clean --tmux-echo` via `lib/find_binary.sh`.
- **`lib/find_binary.sh`** — Shared binary detection: checks `dist/tpack` > `./tpack` > `$PATH` > auto-download.
- **`lib/download_binary.sh`** — Auto-downloads from GitHub Releases to `./tpack`. Opt-out via `TPACK_AUTO_DOWNLOAD=0` or legacy `TPM_AUTO_DOWNLOAD=0`.

### Key Packages (`internal/`)

| Package | Responsibility |
|---|---|
| `config` | Reads tmux options (`@tpack-*`, `@tpm-*` legacy), env vars (`TPACK_PLUGIN_PATH`, `TMUX_PLUGIN_MANAGER_PATH`), resolves config. |
| `plug` | Plugin model, parsing (`set -g @plugin "user/repo#branch"`), path resolution. |
| `manager` | Orchestrates install/update/clean/source operations using git interfaces. |
| `git` | Interface definitions (`Cloner`, `Puller`, `Validator`, `Fetcher`). |
| `git/cli` | Git CLI implementation of those interfaces. |
| `registry` | Plugin registry fetching and searching (YAML from GitHub). |
| `tmux` | Tmux runner (real + mock for tests). Version parsing. |
| `tui` | Bubbles-based interactive TUI (browse, commit viewer, etc.). |
| `ui` | Output abstraction (`ShellOutput` vs `TmuxOutput`). |
| `state` | Persistent state (update timestamps) with advisory file locking. |

## Key Facts & Constraints

### Plugin Path Resolution
Priority order:
1. `TPACK_PLUGIN_PATH` / `TMUX_PLUGIN_MANAGER_PATH` env var (from tmux environment)
2. `$XDG_CONFIG_HOME/tmux/plugins/` if `$XDG_CONFIG_HOME/tmux/tmux.conf` exists
3. `~/.tmux/plugins/` (default)

### Backward Compatibility Contract
**Must not rename** these even if they look like legacy TPM names:
- `@tpm-install`, `@tpm-update`, `@tpm-clean` — Keybinding options (read as fallback)
- `@tpm_plugins` — Legacy plugin list syntax (read alongside `@plugin`)
- `TMUX_PLUGIN_MANAGER_PATH` — Legacy env var (set alongside `TPACK_PLUGIN_PATH`)
- `TPM_AUTO_DOWNLOAD` — Legacy opt-out env var
- `~/.tmux/plugins/tpm/tpack` — Fallback binary path for auto-download detection

### Binary Auto-Download Behavior
When no binary is found locally, `lib/download_binary.sh` downloads to `./tpack` (not `dist/`). The binary is considered "auto-downloaded" when it lives at `<pluginPath>/tpm/tpack`, which enables the self-update background check.

### TUI Keybinding Generation (`cmd/tpack/init.go`)
Keys are bound via `bindPopupKeys()` (tmux ≥3.2) using `tmux display-popup -E` with fallback to `tmux new-window`, or `bindInlineKeys()` (tmux <3.2) using only `new-window`. The TUI command passed to tmux includes `PATH=` quoting for safe nested shell embedding.

### Git Interface with `--end-of-options`
The cloner uses `--end-of-options` to prevent refs starting with `-` being parsed as git options (e.g., commit SHAs from untrusted config). This causes known test failures under Git 2.43+ (`TestCloner_CloneWithCommitSHA` and siblings) — the option is injected in the test harness but not in real usage.

## Development Workflow

### Bootstrap
Run `mise install` once before `make` to install pinned tools. The `mise.toml` pins:
- `go = "1.26"`
- `golangci-lint = "2.12.2"`
- `goreleaser = "2.17.0"`

### Verification
Always run `make lint` and `make test` before completing tasks. Use `make lint-fix` for auto-fixable issues.

### Local Smoke Testing
Copy the built binary for tmux testing: `cp dist/tpack ./tpack` (git-ignored).

### CLI Changes
If modifying CLI behavior, verify the `tpm` shim and `bindings/` scripts still function — they invoke specific commands with specific flags (`--tmux-echo`).

### New Features
Consider whether a legacy `@tpm-*` equivalent option should also be supported for backward compatibility.

## Testing

### Race Detector
Enabled by default in all `make test-*` targets. Requires `CGO_ENABLED=1` and a working C compiler (e.g., `gcc`).

### Local Race Testing Without gcc
Use `Dockerfile.dev`:
```bash
docker build -t tpack-dev -f Dockerfile.dev .
docker run --rm -v "$(pwd)":/app tpack-dev make test-all
```

### Manual Testing Without Race Detector
Run tests directly with `go test` and without the `-race` flag.

### Known Test Failures
`internal/git/cli` has 4 tests that fail under Git 2.43+ due to `--end-of-options` injection. This does not affect the binary's runtime behavior.

### Test Flags
- `-short` skips network-dependent integration tests.
- `-count=1` disables test caching (used in all CI/Makefile targets).
- Integration/E2E tests require a working `tmux` environment.

### Test Pattern: noopRunner
Integration tests use a `noopRunner{}` implementing `tmux.Runner` that returns empty values, allowing tests to run without tmux.

## Code Patterns

### Plugin Parsing (`internal/plug/parse.go`)
Uses regex to extract `@plugin` declarations supporting double-quoted, single-quoted, and unquoted values. The same `extractMatches` helper is used for `source-file` path extraction.

### Git CLI Abstraction (`internal/git/git.go`)
Defines interfaces (`Cloner`, `Puller`, `Validator`, `Fetcher`, `RevParser`, `Logger`) with `internal/git/cli/` implementing them via `exec.CommandContext`. Mock implementations exist in `internal/git/mock.go` for unit testing.

### Config Resolution (`internal/config/resolve.go`)
Uses functional options (`WithFS`, `WithHome`, `WithXDG`, `WithXDGState`) for testability. Reads both `@tpack-*` options and `@tpm-*` legacy fallbacks, preferring current options.

### State Persistence (`internal/state/state.go`)
Uses advisory file locking (`LOCK_EX`/`LOCK_UN` via `syscall.Flock`) to serialize concurrent access from background processes. State stored as YAML in `$XDG_STATE_HOME/tpack/state.yml`.

### UI Output Abstraction (`internal/ui/`)
`ShellOutput` writes directly to stdout/stderr. `TmuxOutput` wraps output in `tmux display-message` for cleaner tmux integration. The `install --tmux-echo` flag selects which to use.
