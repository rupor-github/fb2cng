# Repository Instructions

## Commands

- Use `go tool task ...`; CI runs `go tool task release` on version tags.
- `go tool task` builds the Linux debug binary at `build/fbc`, builds debug tools, compiles tests, and runs Staticcheck.
- `go tool task test` runs package tests and writes coverage under `build/tests_results`.
- Run a focused package/test with `PACKAGES='./convert/pdf' go tool task test -- -run '^TestName$'`.
- `go tool task lint` runs `go tool staticcheck -f stylish -tests=true ./...`.
- `go tool task go-tidy` and `go tool task go-vendor` iterate all release GOOS/GOARCH targets; release builds use `-mod=vendor`, debug/test builds use `-mod=mod`.

## Generated And Downloaded Inputs

- Task deps generate `misc/version.go`, `common/enums_code.go`, hyphenation dictionaries, sentence models, and bundled PDF fonts; these are ignored/generated except for a few checked-in sentence fixtures.
- If editing `common/enums.go`, regenerate `common/enums_code.go` via a Taskfile target such as `go tool task` or `go tool task test`.
- Embedded resources must exist before plain `go test ./...`: `content/text/dictionaries/*.gz`, `content/text/sentences/*.json.gz`, and `convert/pdf/fonts/*.ttf.gz` are downloaded by Taskfile deps.

## Architecture

- Main CLI entrypoint is `cmd/fbc/main.go`; it wires config/logging/reporting, then calls `convert.Run` for `convert` and config dump logic for `dumpconfig`.
- Conversion flow is `convert/run.go` -> `content.Prepare` -> one of `convert/epub`, `convert/kfx`, or `convert/pdf`.
- `content.Prepare` builds the normalized FB2 model once, indexes images/links/footnotes/pages, sets temporary work dirs, and feeds every output format.
- `config/config.yaml.tmpl` is embedded and processed with `gencfg`; user config is merged on top with YAML `KnownFields(true)`, so unknown keys are errors.
- `cmd/mhl` is a Windows MyHomeLib wrapper around `fbc.exe`; keep its config/templates separate from the main converter config.

## Testing Notes

- Prefer Taskfile tests over raw `go test` because task deps prepare embedded data and generated code.
- Run focused tests with `PACKAGES=<module path> go tool task test -- -run=<Test name>`.
- EPUB integration tests in `convert/epub/epub_test.go` are skipped only with `-short`; normal `go tool task test` includes them.
- Profiling tests are opt-in: `PDF_PROFILE_BOOK=... go test ./convert/pdf -run '^TestPDFProfilePath$'` and `KFX_PROFILE_BOOK=... go test ./convert/kfx -run '^TestKFXProfilePath$'`.
- To produce a test book, run `go run cmd/fbc -d -c build/test.yaml convert -ow --to <epub2|epub3|azw8|pdf> /mnt/d/test/_Test.fb2 /tmp/<temp dir>`.
- For all formats, `fb2cng-report.zip` contains additional debug information after generation.
- To produce debug dumps for AZW8, use a temp directory and run `go run cmd/debug/kfxdump --all --overwrite /mnt/d/<azw8 file>`.

## Validation

- All changes need to be validated with `go tool staticcheck`; `gopls` for changed files should show no warnings.
- Instead of `gofmt`, run `goimports-reviser -format -set-alias -company-prefixes github.com/rupor-github -excludes vendor` on changed files.
- When changes are related to KFX, validate the generated file with `testdata/run_kfx_input.py <file name>`; output should contain only 3 warnings related to `fbc`.
- When changes are related to EPUB, validate generated EPUB2 and EPUB3 files with `testdata/epubcheck.sh <file name>`; output should have no errors or warnings.
- When changes are related to PDF, validate the generated file with `qpdf` and/or `pdfinfo`.

## Workflow Gotchas

- Taskfile assumes Linux and forces `GOTOOLCHAIN=local+path`; the `go.mod` Go version must already be available locally.
- `go tool task` installs pre-commit hooks only if `pre-commit` is available; pre-push also runs `trivy --exit-code 1 fs --ignore-unfixed .`.
- `go tool task release` requires `7z` and creates `release/fbc-*.zip`; Windows release archives include `mhl-connector` as well as `fbc`.
- The repo has both `.git` and `.jj`; do not modify VCS metadata unless explicitly asked.
- Project is using jj-vcs (not git).
