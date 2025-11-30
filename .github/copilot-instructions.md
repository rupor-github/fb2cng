---
applyTo: "**"
---

# Project general coding standards

This project uses Go (Golang) and follows standard community practices, prioritizing simplicity and clarity.

## Naming Conventions
*   **Variables/Functions:** Use `camelCase` for variable names and function names.
*   **Structs/Interfaces:** Use `PascalCase` for struct names and interfaces.
*   **Constants:** Use `ALL_CAPS` for constants.
*   **Acronyms:** Acronyms (like `URL` or `HTTP`) should be all uppercase in names (e.g., `makeHTTPRequest`, not `makeHttpRequest`).
*   **Private/Public:** Use lowercase first letter for package-private items; use uppercase first letter for publicly exported items.

## Code Style and Formatting
*   **Indentation:** Use tabs for indentation, not spaces (standard Go practice).
*   **Formatting:** Always run `go fmt` before committing. Generated code should be explicitly marked and excluded from style checks.
*   **Imports:** Use standard grouping for imports: standard library first, then third-party libraries, then local project packages, separated by blank lines.

## Error Handling
*   **Error Values:** Errors should be the last return value and have type `error`.
*   **Error Wrapping:** Wrap errors using `fmt.Errorf` with `%w` where appropriate to preserve the original error chain.
*   **Panics:** Avoid `panic` except for truly unrecoverable situations (e.g., a critical configuration issue at startup). Use error returns for expected failures.
*   **Logging:** Use uber zap structured logging library for all logging (avoid `fmt.Println` in production code).

## Project Structure
*   **Dependencies:** We use [Go Modules](go.dev) for dependency management.
*   **Testing:** Write unit tests for all new functions. Test files should have the `_test.go` suffix. Use the built-in `testing` package and `go test` runner.

## Copilot Directives
*   **Prefer standard library:** Where possible, prefer the Go standard library over external dependencies.
*   **Concurrency:** When writing concurrent code, prioritize using channels and goroutines following Go idioms.
*   **Code Review:** When performing a code review, focus on idiomatic Go practices, test coverage, and clear error handling.
*   **Temporary artifacts:** Always generate temporary artifacts (like test binaries) in the /tmp directory