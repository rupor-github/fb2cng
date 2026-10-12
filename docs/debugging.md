# Debugging And Validation

This guide covers converter debug reports, the standalone development utilities, output
validators, and performance profiling. Start with `fbc --debug` when investigating a
conversion; use the standalone tools to inspect or transform files already on disk.

## Choose A Tool

| Facility | Input | Purpose |
|---|---|---|
| [`fbc --debug` / `-d`](#converter-debug-reports) | FB2 conversion | Capture logs, parsed/prepared content, output, and format-specific diagnostics |
| [`kfxdump`](#kfxdump) | KFX/AZW8 | Inspect fragments, styles, storyline, margins, resources, and outer envelopes |
| [`kpfdump`](#kpfdump) | Kindle Previewer KDF/KPF | Inspect equivalent Kindle structures and recover clean SQLite data |
| [`kfxcompress`](#kfxcompress) | Ordinary or supported enveloped KFX/AZW8 | Byte-preserving compression/uncompression and plaintext-envelope controls |
| [`run_epubcheck.sh`](#epub-and-kepub) | EPUB/KEPUB | Run external EPUBCheck |
| [`run_kfx_input.py`](#kfx-and-azw8) | KFX/AZW8 | Decode and check the book with external KFXInput |
| [`run_pdf_check.sh`](#pdf-validation) | PDF | Check structure and inspect metadata with qpdf/pdfinfo |
| [Opt-in profile tests](#performance-profiling) | FB2 | Measure KFX/PDF generation stages and write CPU/allocation/heap profiles |

The three standalone utilities (`kfxdump`, `kpfdump`, and `kfxcompress`) do not load the
converter's YAML configuration, enable its reporter, or require `--debug`.

## Converter Debug Reports

```bash
fbc -d convert --to epub3 book.fb2
fbc -d convert --to azw8 --kfx-compression lzma book.fb2
fbc -c config.yaml -d convert --to pdf book.fb2
```

By default, artifacts are written in the current working directory:

- `fbc.log`: file log, also available as `final.log` in the report.
- `fbc-panic.log`: runtime crash output when present; an empty configured panic file is removed
  during normal shutdown.
- `fbc-report.zip`: accumulated diagnostic archive, finalized at command shutdown.

The converter logs the actual report location. If a configured report/log destination cannot
be opened, it can fall back to a temporary file. Artifact paths are independent of the
conversion destination and can be changed through [logging/reporting configuration](#logging-and-artifact-paths).

### Common Archive Contents

| Entry | Contents |
|---|---|
| `MANIFEST` | Archive entry names, timestamps, and original/actual filesystem paths |
| `final.log` | Full file log, including invocation arguments, version, timings, and errors |
| `panic.log` | Crash-output capture when available |
| `config/<config-name>` | Processed, merged configuration when an external YAML file was supplied |
| `fbc-<book-id>/book.fb2` | Parsed input XML written back to disk; not necessarily byte-identical to the original FB2 |
| `fbc-<book-id>/book.pristine.txt` | Readable parsed book tree before content preparation |
| `fbc-<book-id>/book.prepared.txt` | Prepared book tree and image/ID/link/footnote indexes |
| `result-<book-id>.<extension>` | Successfully published output file |

The application name in these entries is currently `fbc`. A batch conversion can contain
multiple book directories and results. The report captures the book content and generated
resources needed for diagnosis; tree dumps summarize binary resources rather than printing
their payloads.

All per-book debug artifacts use fixed, short ASCII `book.*` filenames inside the
book-ID directory, independently of the original filename or title. The source name is
still recorded in logs and continues to control output-name templates.

Book working directories are temporary. They are archived and removed at normal report
finalization; without reporting, they are removed after conversion. Available artifacts
depend on which stages completed. A preparation failure can remove that book's working
directory, and missing files may remain listed in `MANIFEST`. Failures before reporter
initialization, such as invalid configuration, may have no report at all.

The archived configuration is captured before conversion CLI overrides. For example,
`--kfx-compression none` may override an archived `compression: lzma`; the log and KFX debug
text record the effective compression choice and its source.

### Format-Specific Artifacts

#### KFX/AZW8

These files are stored inside the book's report directory:

| Artifact | Use |
|---|---|
| `book.debug.txt` | Container summary and full fragment dump, with effective compression choice/source |
| `book.stylesheet.css` | Parsed stylesheet used for Kindle style conversion, when available |
| `book.style-trace.txt` | CSS mapping, normalization, inheritance, merging, style registration, and assignments |
| `book.envelope.txt` | Outer frame/index/page details for compressed conversion |
| `book.inner.kfx` | Exact original CONT bytes before compression |

These diagnostics use fixed, short ASCII names inside `fbc-<book-id>/`, which already
identifies the book. Their names are independent of the output filename, its length or
characters, and temporary random suffixes. `book.envelope.txt` and `book.inner.kfx`
are added for compressed output. The published result remains at the archive root as
`result-<book-id>.<extension>`.
Outer envelope compression and inner **container compression** are reported separately.
Wrapping a book does not change the inner compression/DRM fields, directory offsets, or hashes.

#### PDF

| Artifact | Use |
|---|---|
| `book.stylesheet.css` | Parsed stylesheet, when available |
| `book.pdf-style-trace.txt` | Defaults, media-query decisions, CSS mappings, resolved styles, drop caps, margins, and assignments |
| `book.pdf-structure-plan.json` | Content/structure plan used to build the document |
| `book.pdf-resolved-styles.json` | Effective typography, spacing, colors, borders, and pagination settings |
| `book.pdf-text-blocks.json` | Ordered blocks with IDs, depth, classes, images, and text |
| `book.pdf-printed-footnotes.json` | Printed-footnote placement and continuation diagnostics |
| `book.pdf-layout-pages.json` | Laid-out pages and their content geometry |
| `book.pdf-line-glyphs.json` | Line/glyph diagnostics for text positioning and font use |
| `book.pdf-justification.json` | Justification decisions and spacing |
| `book.pdf-images.json` | Image placement diagnostics |
| `book.pdf-links.json` | Link targets and placement diagnostics |
| `book.pdf-fonts.json` | Font resources, outline/PDF font types, embedding/subsetting sizes, and glyph/CID mappings |

#### EPUB/KEPUB And TXT/Markdown

Use the common parsed/prepared trees, logs, and published result. EPUB/KEPUB packages contain
their XHTML, stylesheets, navigation, and resources for direct inspection. TXT/Markdown
outputs can be inspected directly. These formats do not have a separate standalone dump
utility or the KFX/PDF diagnostic suites above.
An early EPUB/KEPUB generation failure can also leave the partial package
`book.epub.tmp` in the book directory; it is removed after a successful generation.

### Logging And Artifact Paths

```yaml
logging:
  console:
    level: normal
  file:
    level: debug
    mode: overwrite
    destination_template: "{{ .AppName }}-{{ .Unique }}.log"
    panic_destination_template: "{{ .AppName }}-{{ .Unique }}-panic.log"
reporting:
  destination_template: "{{ .AppName }}-{{ .Unique }}-report.zip"
```

- Console and file levels accept `none`, `normal` (INFO and higher), or `debug`.
- Without `--debug`, logging still follows configuration; default file logging is `debug`.
- `--debug` forces file logging to `debug` and file mode to `overwrite`, even when file
  logging was configured differently. It leaves console verbosity as configured.
- Console messages below ERROR, including warnings, go to stdout; errors go to stderr.
- File mode normally accepts `append` or `overwrite`. The report itself is created anew;
  ordinary conversion's `--overwrite` flag does not control logs or report replacement.
- `.Unique` helps keep artifacts from successive or concurrent invocations separate.

Artifact templates are resolved before FB2 parsing. Available fields are `.Context`,
`.AppName`, `.PID`, `.Hostname`, `.Started`, `.Unique`, `.Command`, `.Format`, `.SourceFile`,
and `.OutputDir`; book metadata is unavailable here. Resolved artifact parent directories
are created automatically.
See [Configuration](config.md#logging-and-reports) and the embedded template for defaults.

To put the file log and debug report alongside a single-book result:

```yaml
logging:
  file:
    mode: overwrite
    destination_template: '{{ .OutputDir }}/{{ .AppName }}.log'
    panic_destination_template: '{{ .OutputDir }}/{{ .AppName }}-panic.log'
reporting:
  destination_template: '{{ .OutputDir }}/{{ .AppName }}-report.zip'
```

`.OutputDir` is the absolute `DESTINATION`, the parent of `--output-file`, or the current
directory when output defaults there. For batches or metadata-derived output subdirectories,
artifacts go in the destination root; the report remains per invocation. The ZIP is produced
only with `--debug`, while file logging also works without it.

For MyHomeLib, `debug: true` in the separate `connector.yaml` passes the debug flag to fbc;
`log_destination` controls the connector's own log. See [MyHomeLib Integration](mhl.md).

## Build The Development Utilities

Run commands from the repository root with the Go toolchain required by `go.mod`.
On a fresh checkout, the full development task prepares generated/embedded resources and
builds the converter and tools:

```bash
go tool task
```

After setup, rebuild only the tools with:

```bash
go tool task debug-tools
```

This Linux development target produces `build/kfxdump`, `build/kpfdump`, and
`build/kfxcompress`. They are not bundled in normal release archives. Manual builds for
another host are possible after preparing resources, for example:

```bash
go build -mod=mod -o build/kfxdump ./cmd/debug/kfxdump
```

Each utility supports `--help`. Put options before positional paths: the dump tools and
each kfxcompress operation use Go's standard flag parser, which stops at the first positional
argument. Single-dash forms such as `-all` work as well as `--all`.

## kfxdump

Reads uncompressed KFX/AZW8 (ordinary CONT) and supported LZMA-compressed or uncompressed
plaintext ProtectedData envelopes, independently of `.kfx`/`.azw8` filename extensions.
Semantic dumps automatically decode enveloped input.

```text
build/kfxdump [OPTIONS] FILE [OUTDIR]
```

At least one dump/extraction option is required.

| Option | Output | Contents |
|---|---|---|
| `--dump` | `<stem>-dump.txt` | File summary, container, symbols, and all fragments; also prints container statistics |
| `--resources` | `<stem>-resources.zip` | Nonempty raw image/media (`$417`) and font (`$418`) bytes, with detected extensions |
| `--styles` | `<stem>-styles.txt` | Style (`$157`) fragments in readable CSS-like form, with direct/style-event usage |
| `--storyline` | `<stem>-storyline.txt` | Storyline (`$259`) fragments with expanded symbols, content/resource references, and styles |
| `--margins` | `<stem>-margins.txt` | Focused vertical-margin tree for layout comparisons |
| `--envelope` | `<stem>-envelope.txt` | Protocol/import versions, sizes/savings, frame/index/trailer offsets, and per-page headers |
| `--extract-inner` | `<stem>-inner.kfx` | Save the exact decoded CONT as a separate file; ordinary CONT input is copied unchanged |
| `--all` | Six diagnostic files | Enables `--dump`, `--resources`, `--styles`, `--storyline`, `--margins`, and `--envelope`; add `--extract-inner` to save CONT too |
| `--overwrite` | — | Replace existing requested dump files |

`<stem>` is the input basename without its final extension. Outputs default to the input
directory; `OUTDIR` redirects them and must already exist. Dumps are written individually,
so an error can leave earlier requested outputs in place. These files are independent of
the converter's ZIP report.

For a supported compressed `compressed.azw8`, `--all` decompresses the complete inner book
in memory and uses it to produce:

```text
compressed-dump.txt
compressed-resources.zip
compressed-styles.txt
compressed-storyline.txt
compressed-margins.txt
compressed-envelope.txt
```

The envelope report describes the original compressed file; the other dumps describe its
decoded book. In-memory decompression is automatic and does not save `compressed-inner.kfx`.
Add `--extract-inner` when you also want that byte-exact, ordinary CONT file on disk:

```bash
build/kfxdump --all --extract-inner compressed.azw8
```

```bash
mkdir -p /tmp/fbc-inspect
build/kfxdump --all book.azw8 /tmp/fbc-inspect
build/kfxdump --styles --margins --overwrite book.azw8 /tmp/fbc-inspect
build/kfxdump --envelope --extract-inner compressed.azw8 /tmp/fbc-inspect
```

`--envelope` alone checks supported structure and page headers without decompressing LZMA
payloads or parsing book fragments. A successful inspection is therefore not a full page-data
verification. For raw CONT it states that no outer envelope is present. Every invocation
prints a file-level summary; **container compression: 0** can correctly coexist with outer LZMA.

The envelope reader supports one version-2 plaintext frame, decoded pages of 4, 16, 32, or
64 KiB, and either all LZMA or all uncompressed pages. It follows the size declared in the
file, including legacy 4-KiB output. Decoded buffering is limited to 1 GiB;
unsupported variants/encrypted records are rejected. It does not decrypt Kindle DRM.

## kpfdump

Reads Kindle Previewer `.kdf` SQLite data or a `.kpf` ZIP containing a `.kdf` entry. It strips
the Kindle SQLite fingerprint records before reading fragments. Already-clean SQLite input
is accepted and reported as having no fingerprints to remove.

```text
build/kpfdump [OPTIONS] FILE.kdf|FILE.kpf [OUTDIR]
```

`--dump`, `--resources`, `--styles`, `--storyline`, `--margins`, and `--overwrite` have the
same output naming/purpose as kfxdump. At least one dump/extraction option is required.
Additional behavior:

| Option | Output | Contents |
|---|---|---|
| `--sqlite` | `<stem>.sqlite` | Clean database after fingerprint removal |
| `--all` | Semantic dumps/resources plus SQLite | Includes `--sqlite`; there are no KFX envelope/inner-extraction options |

```bash
build/kpfdump --all previewer-book.kpf /tmp/fbc-inspect
build/kpfdump --sqlite previewer-book.kdf /tmp/fbc-inspect
build/kpfdump --styles --storyline --margins previewer-book.kpf /tmp/fbc-inspect
```

Prefer KPF when extracting resources: path-based payloads can be resolved from the ZIP.
A standalone KDF may reference external resources it does not contain; those bytes are
unavailable to resource extraction and the tool prints a note. Missing resources in a KPF
produce warnings. `--sqlite` alone does not require semantic fragment parsing.

Use matching kfxdump/kpfdump style, storyline, and margin reports to compare converter output
with a Previewer-produced book. IDs and metadata can differ between producers, so focused
reports are often easier to compare than full fragment dumps.

## kfxcompress

Adds or removes lossless LZMA compression for existing KFX/AZW8 files without running FB2
conversion or rebuilding fragments. It wraps the original CONT stream or recovers it
byte-for-byte, preserving inner symbols, offsets, metadata, resources, and payload hashes.

```text
build/kfxcompress compress [--uncompressed-envelope] [--overwrite] INPUT OUTPUT
build/kfxcompress uncompress [--overwrite] INPUT OUTPUT
```

Exactly one input and one explicit output path are required. Extensions are retained as
supplied; there is no automatic output naming or recursive processing.

| Operation | Accepted input | Output |
|---|---|---|
| `compress` | Ordinary CONT or supported uncompressed plaintext envelope | LZMA plaintext envelope |
| `compress --uncompressed-envelope` | Ordinary CONT only | Diagnostic envelope with uncompressed pages |
| `uncompress` | Supported compressed or uncompressed plaintext envelope | Exact original ordinary CONT |

Already-compressed input to `compress` and ordinary CONT input to `uncompress` are errors,
not idempotent copies. Unsupported profiles or encryption are errors. The format/size limits
are the same as [kfxdump's envelope support](#kfxdump).

```bash
build/kfxcompress compress book.azw8 compressed.azw8
build/kfxcompress uncompress compressed.azw8 original.azw8
build/kfxcompress compress --uncompressed-envelope book.azw8 control.azw8
build/kfxcompress uncompress --overwrite compressed.azw8 compressed.azw8
```

The utility reports sizes, savings, mode, page count, and elapsed time to stderr. Compression
is honored even if it enlarges an incompressible book. The plaintext control separates envelope
layout compatibility from LZMA behavior; it is not a conversion configuration mode.

Both compression and diagnostic plaintext wrapping use the same [adaptive page-size
policy](formats/kindle.md#adaptive-page-size) as conversion: 4/16/32/64 KiB based on the
decoded CONT size, aiming for 256 pages subject to a 64-KiB cap. Inspection reports the
actual page size stored in the envelope; uncompression preserves bytes independently of
which supported size was used.

Destination parents must exist. Existing output, including same-path input, requires
`--overwrite`. Publication uses a sibling temporary file and an atomic final operation;
no-overwrite publication also checks that the destination is still absent. Errors or
cancellation before publication preserve existing output. Ctrl+C is supported by both
kfxcompress and kfxdump.

## Output Validators

Validation scripts live in `testdata`; they use external tools rather than the converter's
own dump routines. Structural checks complement opening the result in the intended reader.

### EPUB And KEPUB

```bash
testdata/run_epubcheck.sh book-epub2.epub
testdata/run_epubcheck.sh book-epub3.epub
testdata/run_epubcheck.sh book.kepub.epub
```

EPUBCheck itself requires a Java runtime and a distribution containing `epubcheck.jar`.
SDKMAN is optional. When its initialization script is readable, the wrapper loads
`${SDKMAN_DIR}/bin/sdkman-init.sh` (or `~/.sdkman/bin/sdkman-init.sh` if `SDKMAN_DIR` is unset).
Otherwise it uses Java already on `PATH`. It checks that `java` is available before locating
EPUBCheck. Maven is not required.

Set `EPUBCHECK_PATH` to the directory containing `epubcheck.jar`, or `BOOK_TESTS_PATH` to
the parent directory containing an `epubcheck` or `EPUBCheck` subdirectory. Common local
checkouts are also searched. Additional arguments are passed to EPUBCheck with its default
profile. Expected ordinary EPUB2/3 and KEPUB output has no errors or warnings. Options such as
the Adobe page map described in the configuration template can deliberately fall outside
normal EPUBCheck expectations.

### KFX And AZW8

```bash
testdata/run_kfx_input.py book.azw8
testdata/run_kfx_input.py --kfxinput /path/to/KFXInput --kfxcompress build/kfxcompress compressed.azw8
```

`kfxlib` comes from the [KFX Input conversion plugin for calibre](https://www.mobileread.com/forums/showthread.php?t=291290).
Download and unpack the plugin, or use a source checkout containing its `kfxlib/` directory.
Point `--kfxinput` or `KFXINPUT_PATH` at the directory containing `kfxlib/`.

The script first tries an already-importable package, then searches `--kfxinput PATH`,
`KFXINPUT_PATH`, and `BOOK_TESTS_PATH` (whose KFXInput child is searched), followed by common
local checkout paths.

On DRMION magic, the script calls kfxcompress to decode the supported plaintext envelope
into a temporary CONT file, validates that file, and cleans up the temporary directory.
Utility selection is `--kfxcompress PATH`, then `KFXCOMPRESS_PATH`, then repository
`build/kfxcompress`, then PATH. Ordinary CONT follows the direct validation path.

The script prints debug messages, warnings/errors, and a final summary. Errors or a fatal
exception yield a nonzero exit status; warnings alone do not. For generated books, the expected
baseline is the three warnings about the fbc creator version, file creator, and generator,
with no errors or additional warnings.

This checks decoded book semantics. Use KPV or the intended reader to verify the outer
envelope opens correctly, content renders, and navigation works. The DRMION signature alone
does not imply encryption.

### PDF Validation

```bash
testdata/run_pdf_check.sh book.pdf
testdata/run_pdf_check.sh --require-metadata book.pdf
```

The wrapper requires `qpdf` and `pdfinfo` (commonly supplied by `poppler-utils`). It runs
`qpdf --check` and prints pdfinfo output. `--require-metadata` additionally requires an XMP
metadata stream; use it for the converter's metadata-bearing PDF output. Multiple PDF paths
are accepted. Inspect the [PDF report artifacts](#pdf) for layout/font problems that structural
validation alone cannot explain.

## Fuzz Testing

Go's fuzz targets replay their explicit seeds and checked-in regression inputs during
normal package tests:

```bash
PACKAGES='./convert/kfx/envelope' go tool task test
```

To actively generate and mutate inputs, run the manual fuzz task:

```bash
go tool task fuzz
FUZZ_TIME=5m FUZZ_PARALLEL=2 go tool task fuzz
go tool task fuzz -- -v
```

The task prepares the usual generated/embedded inputs, then runs `FuzzIonReader` and
`FuzzInspect` sequentially. Defaults are **15 seconds per target** and **two workers**;
`FUZZ_TIME` and `FUZZ_PARALLEL` override those settings. Allow extra wall-clock time for
task dependencies and compilation. Active fuzzing is opt-in and does not change release workflows.

This task omits coverage profiles because Go does not allow `-coverprofile` together
with `-fuzz`. The ordinary `test` task continues to collect coverage.

The seeds include typed-null imports, truncated values, oversized scalar declarations,
excessive nesting, invalid envelope offsets, plaintext/LZMA multi-page envelopes, and
native reference fixtures. Inputs and decoded payloads are bounded to 64 KiB. Successfully
decoded envelopes must survive a byte-exact decode/encode/decode round-trip.

On a failure, Go minimizes the input and writes a reproducer under
`convert/kfx/envelope/testdata/fuzz/<target>/`. Keep that file with the fix so normal tests
replay it, and use the `-run` command printed by Go to reproduce the failure through
`PACKAGES='./convert/kfx/envelope' go tool task test -- ...`.
Interesting inputs that do not fail are cached by Go locally, allowing later fuzz runs
to continue exploring them.

## Performance Profiling

Profiling tests are skipped unless an input FB2 path is supplied. They prepare shared content
before starting profiles and disable converter debug reporting, so measurements focus on
the selected generation stage.

```bash
KFX_PROFILE_BOOK=/path/to/book.fb2 KFX_PROFILE_TARGET=generate \
  PACKAGES='./convert/kfx' go tool task test -- -run '^TestKFXProfilePath$' -count=1 -timeout=30m -v

PDF_PROFILE_BOOK=/path/to/book.fb2 PDF_PROFILE_TARGET=layout \
  PACKAGES='./convert/pdf' go tool task test -- -run '^TestPDFProfilePath$' -count=1 -timeout=30m -v
```

| Variable suffix | KFX (`KFX_PROFILE_…`) | PDF (`PDF_PROFILE_…`) |
|---|---|---|
| `BOOK` | Required FB2 path | Required FB2 path |
| `TARGET` | `styles`, `images`, `storyline`, `fragments`, `serialize`, `generate` (default) | `collect`, `styles`, `layout`, `builddoc`, `generate` (default) |
| `OUT` | Default temporary-directory child `fb2cng-kfx-profile` | Default temporary-directory child `fb2cng-pdf-profile` |
| `CONFIG` | Config path; otherwise repository `build/test.yaml` if present, then defaults | Same |
| `FOOTNOTES_MODE` | Optional `default`, `float`, or `floatRenumbered` override | Not supported |

Each selected stage writes `<target>.cpu.pprof`, `<target>.allocs.pprof`, and
`<target>.heap.pprof`, and logs elapsed time/allocation deltas. The output directory is
created automatically. The `generate` target also writes `profile-output.kfx` or
`profile-output.pdf`; repeated runs replace files with the same names.
The KFX `serialize` target measures inner CONT serialization; `generate` also includes
outer compression when enabled in the selected configuration.

```bash
go tool pprof -top /tmp/fb2cng-kfx-profile/generate.cpu.pprof
go tool pprof -sample_index=alloc_space -top /tmp/fb2cng-kfx-profile/generate.allocs.pprof
```

PDF also provides opt-in `BenchmarkPDFProfileCollectContent`, `BenchmarkPDFProfileStyles`,
`BenchmarkPDFProfileLayout`, `BenchmarkPDFProfileBuildDocument`, and `BenchmarkPDFProfileGenerate`:

```bash
PDF_PROFILE_BOOK=/path/to/book.fb2 PACKAGES='./convert/pdf' \
  go tool task bench -- -bench '^BenchmarkPDFProfile' -benchmem -benchtime=1x
```

For envelope-only measurements:

```bash
PACKAGES='./convert/kfx/envelope' go tool task bench -- -bench '^BenchmarkEncode$' -benchmem
```

Package selections in Taskfile commands are comma-separated when more than one package is
supplied. `go tool task escape -- ./convert/kfx` runs Go escape analysis for allocation
investigation.

## Investigation Workflow

1. Reproduce with `fbc -d` and retain the exact command, converter version, and configuration.
2. Open `MANIFEST` and `final.log`; find the book directory and published result.
3. Compare `book.pristine.txt` and `book.prepared.txt` when content, IDs, links, images, or footnotes differ
   from the source expectations.
4. For styling/layout, inspect KFX CSS/style traces and focused dumps, or PDF style/layout/
   glyph/font JSON. Compare KPF/KFX focused reports when using Previewer as a reference.
5. Run the corresponding external validator, then check reading/navigation in the target viewer.
6. For compressed KFX, inspect the envelope and extract inner CONT to distinguish wrapping
   problems from book-generation problems. For slow conversions, use the opt-in profiles.

When reporting an issue, include the command line, converter version, relevant configuration,
the complete debug report ZIP file (`fbc-report.zip` by default), validator output, and the
reader/version used to reproduce it. Attach the full ZIP archive rather than selected files
or excerpts from it.
