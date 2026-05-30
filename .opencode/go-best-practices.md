---
name: go-best-practices
description: "Idiomatic modern Go patterns and project conventions for AI-assisted development."
updated: "2026-05-23"
---

# Go Best Practices Skill

Use this skill when writing, editing, reviewing, refactoring, debugging, or validating Go code.

## Required finish steps for Go edits

After changing `.go` files:

1. Format every changed Go file with `goimports-reviser` only:
   ```bash
   GOFLAGS=-mod=mod goimports-reviser -format -set-alias -company-prefixes github.com/rupor-github <changed .go files>
   ```
2. Run `gopls check -severity=hint <changed .go files>` and fix all diagnostics.
3. Run the narrowest relevant validation (`go test`, `go vet`, lint, build, etc.).
   Prefer module-pinned tools via `go tool <name>` when available.
   If `staticcheck` is declared in `go.mod` as a project tool, run `go tool staticcheck ./...` as part of lint validation.

Skip these steps only when no Go files changed.

## Go formatting rule

Prefer `goimports-reviser` for all Go formatting/import cleanup in this project:

```bash
GOFLAGS=-mod=mod goimports-reviser -format -set-alias -company-prefixes github.com/rupor-github <changed .go files>
```

Do **not** run `gofmt -w` directly when `goimports-reviser` is available. `goimports-reviser -format` performs gofmt-compatible formatting and import cleanup, so a separate `gofmt` pass is unnecessary and should be avoided.

If `goimports-reviser` is not available on `PATH`, `gofmt -w <changed .go files>` is an acceptable fallback. When using this fallback, explicitly mention that `goimports-reviser` was unavailable so import cleanup may be incomplete.

## Critical rules

- **Line length:** keep Go source lines at or below 150 characters; wrap long calls, literals, and expressions.
- **Errors:** use `errors.New("literal string")` when no formatting is needed; use `fmt.Errorf` for formatted or wrapped errors.
  Wrap with context using `fmt.Errorf("...: %w", err)`; check with `errors.Is` / `errors.As`; aggregate failures with `errors.Join`.
- **Context:** pass `context.Context` as the first parameter; never store it in structs.
- **Concurrency:** every goroutine needs a bounded lifetime; prefer `errgroup.WithContext` for coordinated work.
  Use `sync.WaitGroup.Go` when a plain wait group is enough.
- **Logging:** use structured `go.uber.org/zap` logging for server applications; if a logger is passed to a function, pass it last.
- **APIs:** accept interfaces, return concrete types; keep interfaces close to consumers.
- **Pointers:** avoid generic `Ptr` / `ToPtr` helpers for simple value initialization; use direct language constructs instead.
- **Modern stdlib:** prefer `slices`, `maps`, `iter.Seq`, integer `range`, and Go 1.22+ loop-variable semantics where supported by the module.
- **Tests:** use table-driven tests with `t.Run` subtests.
- **Initialization:** avoid `init` and package globals for dependency wiring; use explicit constructors.
- **Configuration:** for services, prefer `xandr-tools-gencfg-golang`: embed `config.yaml.tmpl`, process defaults,
  reject unknown YAML fields with `KnownFields(true)`, then sanitize and validate.
- **Modernization:** on Go 1.26+ modules, consider `go fix ./...` for legacy idioms, then review the diff and validate.

## Anti-patterns

- Panicking for expected errors.
- Ignoring errors without an explicit reason.
- Global variables for dependency injection.
- Naked goroutines without cancellation or another exit path.
- Returning interfaces from constructors or package APIs.
- Hand-rolled collection helpers when `slices` / `maps` fit.
- Hand-rolled service configuration loading when the shared config generator fits.
