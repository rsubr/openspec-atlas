# Repository Guidelines

## Project Structure & Module Organization
This repository is a small Go CLI. The executable entrypoint lives at `cmd/openspec-atlas/main.go`. Scanner and parser internals live under `internal/`, with language mappings and tree-sitter queries in `internal/languages.go`. Helper scripts include `build.sh` for release binaries. Generated outputs belong in `dist/` and local scan results such as `structure.json` should stay out of commits unless intentionally updating an example.

## Build, Test, and Development Commands
- `go build ./cmd/openspec-atlas` builds the local CLI for the current platform.
- `go test ./...` runs all Go tests; keep this passing even if coverage is currently light.
- `gofmt -w cmd/openspec-atlas/*.go internal/*.go` formats changed Go files. Use `gofmt` before opening a PR.
- `./build.sh` produces static Linux binaries in `dist/`.
- `go run ./cmd/openspec-atlas -o structure.json .` runs the CLI directly from source and writes an atlas JSON file.

## Coding Style & Naming Conventions
Follow standard Go style: tabs for indentation, `gofmt` formatting, and mixedCaps identifiers. Keep parsing rules data-driven in `internal/languages.go` instead of scattering language-specific behavior across the package. Prefer small helper functions, explicit error handling, and clear flag names such as `-o` and `-all`. Shell scripts should use `bash`, `set -euo pipefail`, and uppercase variable names for script-level constants.

## Testing Guidelines
Add table-driven Go tests in `*_test.go` files beside the code they cover. Focus on parser extraction behavior, ignored-path handling, and CLI flag behavior. For scanner changes, validate both `go test ./...` and a direct source run such as `go run ./cmd/openspec-atlas -o /tmp/atlas.json .` to confirm the output shape is still correct.

## Commit & Pull Request Guidelines
Recent commits use short imperative subjects like `Add generalized annotation/decorator extraction`. Keep that pattern: one line, present tense, no trailing period. PRs should describe the user-visible effect, list validation steps, and call out grammar or query changes explicitly. Include example output snippets or a sample `structure.json` diff when changing extraction behavior.

## Release & Configuration Notes
Go 1.22+ is required. Cross-compiling ARM64 releases also requires `aarch64-linux-gnu-gcc`, matching the GitHub Actions workflow in `.github/workflows/build.yml`.
