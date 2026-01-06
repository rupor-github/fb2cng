---
applyTo: "**"
---

This project uses Go (Golang) >= 1.25 and follows standard community practices, strongly prioritizing simplicity and clarity.

These coding principles are mandatory:

1. Naming Conventions
- Use `camelCase` for variable names and function names.
- Use `PascalCase` for struct names and interfaces.
- Use `ALL_CAPS` for constants.
- Acronyms (like `URL` or `HTTP`) should be all uppercase in names (e.g., `makeHTTPRequest`, not `makeHttpRequest`).
- Use lowercase first letter for package-private items; use uppercase first letter for publicly exported items.
- Use descriptive-but-simple names.

2. Code Style and Formatting
- Use tabs for indentation, not spaces (standard Go practice).
- Use `goimports-reviser -format -company-prefixes github.com/rupor-github -excludes vendor ./...` instead of `gofmt`. Generated code should be explicitly marked and excluded from style checks.
- Prefer early funcion exits. When possible prefer if/then to full if/then/else to avoid nesting.
- Prefer usage of `go stdlib` except for logging. 
- Use uber zap structured logging library for all logging (avoid `fmt.Println` in production code).
- Always use latest Go features.
- Errors should be the last return value and have type `error`.
- Wrap errors using `fmt.Errorf` with `%w` where appropriate to preserve the original error chain.
- Avoid `panic` except for truly unrecoverable situations (e.g., a critical configuration issue at startup). Use error returns for expected failures.

3. Project Structure
- Use [Go Modules](go.dev) for dependency management.
- Write unit tests for all new functions. Test files should have the `_test.go` suffix. Use the built-in `testing` package and `go test` runner.
- Do not generate benchmarks.

4. Quality
- Favor deterministic, testable behavior.
- Keep tests simple and focused on verifying observable behavior.

5. Copilot Directives
- Never build anything in project directory, use /tmp. Always generate temporary artifacts in the /tmp directory only
- Do not ask for permission to run 
  `go`, `go build`, `go vet`, `go test`, `go tool`, `go mod`, `go run`, 
  `sed`, `awk`, `grep`, `head`, `tail`, 
  `staticcheck`, `python`, `python3`. 
  Assume that they are always available.

Your goal: produce code that is predictable, debuggable, and easy to rewrite or extend...
