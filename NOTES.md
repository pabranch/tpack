# Notes

Working observations from the tpack codebase. Not a design doc — a parking lot
for facts that came up while building, testing, and reviewing branding.

---

## TPM references — audit of user-facing vs. internal

Audit performed against the working tree after the dedup-fix build
(`v1.2.0-3-g91cd08a`). All hits for the string `TPM` (case-sensitive) under
`cmd/`, `internal/`, `bin/`, `bindings/`, `lib/`, `docs/`, and `tpm`.

### Result

**No user-facing string in the binary or runtime output brands tpack as "TPM"
incorrectly.** Every visible output path that mentions TPM does so
intentionally in a "drop-in replacement for TPM" framing, or names a tmux
option / env var that *must* keep the legacy `@tpm-` prefix to avoid breaking
existing user configs.

The closest user-facing string to a cosmetic issue is:

- `internal/ui/tmux_output.go:52` — `"TMUX environment reloaded."`
  - All-caps `TMUX` here refers to the *tmux multiplexer*, not TPM the
    plugin manager. Shown in the message bar after `prefix + I` etc.
  - Not a TPM-vs-tpack bug, but the all-caps rendering is visually loud next
    to tpack's lowercase branding. Optional: lowercase to
    `"Tmux environment reloaded."`.

### Categorised inventory

#### 1. Intentional / correct — keep as-is

| File:line | String | Why it stays |
|---|---|---|
| `cmd/tpack/root.go:18` | `"tpack is a drop-in replacement for TPM (Tmux Plugin Manager)."` | `tpack --help` long description. Accurate framing. |
| `README.md`, `docs/index.md`, `docs/getting-started/*`, `docs/troubleshooting/faq.md` | Various — all about migration, drop-in replacement, acknowledgments | Documents tpack's relationship to TPM; required for users moving off TPM. |
| `internal/manager/source.go:54`, `internal/git/cli/cloner.go:75`, `internal/plug/resolve.go:35` | "TPM" in `//` comments | Internal notes about TPM's behavior that tpack emulates or diverges from. Fine. |

#### 2. Backward-compat surface — must NOT be renamed

These are part of tpack's public compatibility contract with existing TPM
users. Renaming any of them silently breaks configs in the wild.

| File:line | Identifier | Purpose |
|---|---|---|
| `internal/config/config.go:25` | `LegacyInstallKeyOption = "@tpm-install"` | Read fallback for `@tpack-install` |
| `internal/config/config.go:26` | `LegacyUpdateKeyOption  = "@tpm-update"` | Read fallback for `@tpack-update` |
| `internal/config/config.go:27` | `LegacyCleanKeyOption   = "@tpm-clean"` | Read fallback for `@tpack-clean` |
| `internal/config/config.go:48` | `LegacyAutoDownloadEnvVar = "TPM_AUTO_DOWNLOAD"` | Env-var fallback for `TPACK_AUTO_DOWNLOAD=0` |
| `internal/config/tmuxconf.go:11,17` | `runner.ShowOption("@tpm_plugins")` | Legacy `set -g @tpm_plugins '…'` list syntax |
| `lib/download_binary.sh:11` | Both `TPACK_AUTO_DOWNLOAD` and `TPM_AUTO_DOWNLOAD` are honoured | Shell-side env-var fallback |

#### 3. Path / fixture strings — not user-visible

These are filesystem paths and test fixtures that happen to contain the
substring `tpm`:

- `cmd/tpack/init.go:143,190` — `~/.tmux/plugins/tpm/tpack` fallback path
- `cmd/tpack/init_test.go:172–229` — same path in test fixtures
- `internal/config/writer_test.go:11,27,74,92,102` — fixture plugin named
  `tmux-plugins/tpm`
- `internal/config/tmuxconf_test.go:14,22,32,40` — fixture plugin / option

The plugin-name "tmux-plugins/tpm" is a real GitHub repo (the original TPM
project) and tpack intentionally reverse-resolves and migrates it.
Test fixtures should keep using this canonical name.

### Auditor's process (re-runnable)

```bash
# Branded strings inside compiled output (what users actually see)
strings dist/tpack | grep -E 'TPM|Tmux Plugin Manager'

# Source-side audit — excluding comments and tests
grep -rn 'TPM' --include='*.go' cmd/ internal/ bin/ bindings/ lib/ \
  | grep -v -E '\.git/|// |_test\.go'

# What users type into their configs (forward + backward compat)
grep -rEn '@tpm[-_]|@tpack' --include='*.go' --include='*.md' \
  --include='*.sh' . | grep -v '\.git/'
```

---

## Build & install pipeline notes

- `make build` produces `dist/tpack`. `lib/find_binary.sh` searches
  `$root_dir/dist/tpack` first, then `$root_dir/tpack`, then `$PATH`,
  then auto-downloads from GitHub Releases.
- For the dev workflow (build → smoke test in tmux), copy `dist/tpack` to
  the project root: `cp dist/tpack ./tpack`. Confirmed working with the
  dedup fix.
- The project's own `.gitignore` ignores the root `tpack` binary, so this
  copy is a local-only smoke-test artifact, not something to commit. The
  release binary is produced by GoReleaser per tag.

## Mise / toolchain setup

`mise.toml` pins `go`, `golangci-lint@2.12.2`, `goreleaser@2.17.0`. See `AGENTS.md` for bootstrap instructions.

## Known test breakage (also in `todo.md`)

`internal/git/cli` has 4 tests failing under git 2.43+. Pre-existing, not
introduced by the dedup fix. See `todo.md`.
