# Debugging And Validation

This guide covers converter debug reports, the standalone development utilities, and
external validation/reference tools. Start with `fbc --debug` when investigating a
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
| [`run_kindleunpack.sh`](#kindleunpack) | Unencrypted MOBI/AZW3 | Decode books, inspect headers, and extract content/resources |
| [`run_kindlegen.sh`](#kindlegen-references) | EPUB, OPF, or HTML | Generate a reference combo MOBI with external KindleGen |

The three standalone utilities (`kfxdump`, `kpfdump`, and `kfxcompress`) do not load the
converter's YAML configuration, enable its reporter, or require `--debug`.

## Converter Debug Reports

```bash
fbc -d convert --to epub3 book.fb2
fbc -d convert --to azw8 --compression lzma book.fb2
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

**Review reports before sharing.** They can contain the full book, generated
output, configuration, and local paths. Share only material you have permission
to distribute; if needed, reproduce the problem with a shareable sample.

All per-book debug artifacts use fixed, short ASCII `book.*` filenames inside the
book-ID directory, independently of the original filename or title. The source name is
still recorded in logs and continues to control output-name templates.

Book working directories are temporary. They are archived and removed at normal report
finalization; without reporting, they are removed after conversion. Available artifacts
depend on which stages completed. A preparation failure can remove that book's working
directory, and missing files may remain listed in `MANIFEST`. Failures before reporter
initialization, such as invalid configuration, may have no report at all.

The archived configuration is captured before conversion CLI overrides. For example,
`--compression none` may override an archived `compression: lzma` for KFX/AZW8 or
`compression: standard` for AZW3. Logs and format-specific debug text record the
effective compression choice and its source.

### Format-Specific Artifacts

#### KFX/AZW8

These files are stored inside the book's report directory:

| Artifact | Use |
|---|---|
| `book.debug.txt` | Container summary and full fragment dump, with effective compression choice/source |
| `book.stylesheet.css` | Parsed stylesheet used for Kindle style conversion, when available |
| `book.style-trace.txt` | CSS mapping, normalization, inheritance, merging, style registration, and assignments |
| `book.envelope.txt` | Outer frame/index/page details for compressed conversion |
| `book.inner.kfx` | Exact original uncompressed book file |

These diagnostics use fixed, short ASCII names inside `fbc-<book-id>/`, which already
identifies the book. Their names are independent of the output filename, its length or
characters, and temporary random suffixes. `book.envelope.txt` and `book.inner.kfx`
are added for compressed output. The published result remains at the archive root as
`result-<book-id>.<extension>`.
Compressed output includes both the published file and the original uncompressed book
for troubleshooting. Compression does not change book content.

#### AZW3

| Artifact | Use |
|---|---|
| `book.azw3.txt` | Native compilation summary and effective compression choice/source |
| `book.source.css` | Publication CSS before native resource-URL lowering |
| `book.stylesheet.css` | Actual compiled stylesheet used by the native book |
| `book.azw3-style-trace.txt` | HTML compatibility transformations, native anchor allocation, CSS page-break decisions, and image/font resource mapping |
| `azw3-input/`, `azw3-compiled/` | Numbered XHTML documents before and after native lowering; filenames are mapped in the document inventory |
| `book.azw3-documents.json` | Document order, byte sizes, and per-document transformations |
| `book.azw3-anchors.json` | Resolved reading anchors and link destinations |
| `book.azw3-backlinks.json` | Final template context for each footnote reference, including generated locations |
| `book.azw3-resources.json` | Font, raster-image, and SVG resource inventory |
| `azw3-resources/` | Checksummed pre-lowering resource bytes referenced by the resource inventory |
| `book.azw3-flows.json`, `book.azw3-records.json` | Native data sizes and checksums for comparison |
| `book.azw3-navigation.json` | TOC, landmarks, page-map entries, start-reading target, and Kindle identity |
| `book.rawml` | Frozen native text and stylesheet/artwork data |
| `book.apnx` | Selected page-map sidecar, when generated |

These are compilation diagnostics, not reader-computed styles or a rendering trace.
Use them to compare source CSS/markup with the compiled result, then check appearance in
the intended Kindle reader. The published AZW3 and selected APNX are also at the archive root.

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

Reads uncompressed and supported compressed KFX/AZW8 files, independently of their filename
extensions. Book inspection automatically decompresses supported input. Encrypted books
are not supported; the tool does not remove Kindle DRM.

```text
build/kfxdump [OPTIONS] FILE [OUTDIR]
```

At least one dump/extraction option is required.

| Option | Output | Contents |
|---|---|---|
| `--dump` | `<stem>-dump.txt` | File summary, container, symbols, and all fragments; also prints container statistics |
| `--resources` | `<stem>-resources.zip` | Embedded image/media and font files, with detected extensions; vector drawings are shown in text dumps, not exported as images |
| `--styles` | `<stem>-styles.txt` | Styles in readable CSS-like form, with usage information |
| `--storyline` | `<stem>-storyline.txt` | Reading content with image references and styles |
| `--margins` | `<stem>-margins.txt` | Focused vertical-margin tree for layout comparisons |
| `--envelope` | `<stem>-envelope.txt` | Compression details, sizes, and savings |
| `--extract-inner` | `<stem>-inner.kfx` | Save the original uncompressed book separately; uncompressed input is copied unchanged |
| `--all` | Six diagnostic files | Enables `--dump`, `--resources`, `--styles`, `--storyline`, `--margins`, and `--envelope`; add `--extract-inner` to save the uncompressed book too |
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
Add `--extract-inner` when you also want the exact uncompressed book file on disk:

```bash
build/kfxdump --all --extract-inner compressed.azw8
```

```bash
mkdir -p /tmp/fbc-inspect
build/kfxdump --all book.azw8 /tmp/fbc-inspect
build/kfxdump --styles --margins --overwrite book.azw8 /tmp/fbc-inspect
build/kfxdump --envelope --extract-inner compressed.azw8 /tmp/fbc-inspect
```

`--envelope` alone inspects compression information without decoding the whole book, so it
does not replace full validation. Unsupported compression variants and excessively large
decoded files are rejected.

## kpfdump

Reads Kindle Previewer `.kdf` data or a `.kpf` package. It can also save a cleaned SQLite
database for further inspection; already-clean input is accepted.

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
produce warnings.

Use matching kfxdump/kpfdump style, storyline, and margin reports to compare converter output
with a Previewer-produced book. IDs and metadata can differ between producers, so focused
reports are often easier to compare than full fragment dumps.

## kfxcompress

Adds or removes lossless LZMA compression for existing KFX/AZW8 files without running FB2
conversion. Uncompression recovers the original book file byte-for-byte.

```text
build/kfxcompress compress [--uncompressed-envelope] [--overwrite] INPUT OUTPUT
build/kfxcompress uncompress [--overwrite] INPUT OUTPUT
```

Exactly one input and one explicit output path are required. Extensions are retained as
supplied; there is no automatic output naming or recursive processing.

| Operation | Accepted input | Output |
|---|---|---|
| `compress` | Uncompressed book or supported uncompressed wrapper | Compressed book, without encryption |
| `compress --uncompressed-envelope` | Ordinary uncompressed book only | Uncompressed diagnostic wrapper |
| `uncompress` | Supported compressed or diagnostic wrapper | Exact original uncompressed book |

Already-compressed input to `compress` and an ordinary uncompressed book passed to `uncompress`
are errors, not idempotent copies. Unsupported compression variants and encrypted input
are rejected, as with [kfxdump](#kfxdump).

```bash
build/kfxcompress compress book.azw8 compressed.azw8
build/kfxcompress uncompress compressed.azw8 original.azw8
build/kfxcompress compress --uncompressed-envelope book.azw8 control.azw8
build/kfxcompress uncompress --overwrite compressed.azw8 compressed.azw8
```

The utility reports sizes, savings, mode, page count, and elapsed time to stderr. Compression
is honored even if it enlarges an incompressible book. Sizing is automatic, as with conversion.
`--uncompressed-envelope` is a troubleshooting option, not a converter configuration setting.

Destination parents must exist. Existing output, including same-path input, requires
`--overwrite`. Errors or cancellation before publication preserve existing output.
Ctrl+C is supported by both kfxcompress and kfxdump.

## Output Validators

The `run_*` scripts live in `testdata`; they use external tools rather than the converter's
own dump routines. Run the examples below from the repository root. Structural checks
complement opening the result in the intended reader. KindleGen generates reference books;
it is not a validator and is not needed for native AZW3 conversion.

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
profile, and its exit status is returned. Expected ordinary EPUB2/3 and KEPUB/KEPUB3
output has no errors or warnings. Options such as
the Adobe page map described in the configuration template can deliberately fall outside
normal EPUBCheck expectations.

### KFX And AZW8

```bash
testdata/run_kfx_input.py book.azw8
testdata/run_kfx_input.py --kfxinput /path/to/KFXInput --kfxcompress build/kfxcompress compressed.azw8
```

The script requires Python 3. `kfxlib` comes from the
[KFX Input conversion plugin for calibre](https://www.mobileread.com/forums/showthread.php?t=291290).
Download and unpack the plugin, or use a source checkout containing its `kfxlib/` directory.
Point `--kfxinput` or `KFXINPUT_PATH` at the directory containing `kfxlib/`.

The script first tries an already-importable package, then searches `--kfxinput PATH`,
`KFXINPUT_PATH`, and `BOOK_TESTS_PATH` (whose KFXInput child is searched), followed by common
local checkout paths.

For supported compressed input, the script calls kfxcompress to create and validate a
temporary uncompressed file, then removes the temporary directory.
Utility selection is `--kfxcompress PATH`, then `KFXCOMPRESS_PATH`, then repository
`build/kfxcompress`, then PATH. Uncompressed books are validated directly.

The script prints diagnostics and a final summary. Errors or a fatal exception yield a
nonzero exit status; warnings alone do not. Three warnings identifying fbc as the producer
are usual. The external checker can also report known compatibility diagnostics for native
vector artwork, so a fixed warning count alone does not determine whether a book is usable.
Do not ignore other errors or warnings; include the complete output when reporting a problem.

Always open the result in Kindle Previewer or your intended reader to check images, text,
and navigation. If vector artwork renders incorrectly, try `document.images.rasterize_svg: true`.

### AZW3 And MOBI

#### KindleUnpack

Use KindleUnpack to inspect an existing unencrypted AZW3 or MOBI book:

```bash
testdata/run_kindleunpack.sh -r -d book.azw3 unpacked/
testdata/run_kindleunpack.sh -r -s -d reference.mobi unpacked-reference/
testdata/run_kindleunpack.sh -r -d -p book.apnx book.azw3 unpacked-with-pages/
```

The wrapper requires Python 3 and a complete KindleUnpack checkout. Set
`KINDLEUNPACK_PATH` to the directory containing `lib/kindleunpack.py`. Otherwise it
uses `${BOOK_TESTS_PATH}/KindleUnpack`; `BOOK_TESTS_PATH` defaults to
`../../../book-tests` relative to the repository root.

Options are passed through and must precede the input filename. `-r` retains raw
data, `-d` includes diagnostic/header information, `-s` splits combo books into
standalone MOBI7 and AZW3 files, and `-p` reads an APNX sidecar. Use
`--epub_version=3` to request EPUB3 reconstruction. If the output directory is
omitted, KindleUnpack derives it from the input path without its extension.

Tool output and exit status are preserved. Inspect the full output even after a
successful exit: warnings, retargeted links, or reconstruction repairs need
investigation. Successful decoding does not establish visual or popup compatibility;
open the original book in the intended Kindle reader as well.

#### KindleGen References

Use KindleGen only when a separately compiled reference book helps investigate a
rendering or compatibility problem:

```bash
testdata/run_kindlegen.sh book.epub -c0 -o reference.mobi
testdata/run_kindlegen.sh publication/book.opf -c1 -verbose -o reference.mobi
testdata/run_kindleunpack.sh -r -s -d publication/reference.mobi unpacked-reference/
```

Provide a compatible KindleGen executable; the wrapper does not download it.
`KINDLEGEN_PATH` accepts either the executable path or a directory containing
`kindlegen`. Otherwise the wrapper uses `${BOOK_TESTS_PATH}/fb2converter/kindlegen`,
with the same `BOOK_TESTS_PATH` default described above.

Arguments are passed directly to KindleGen. `-c0` selects no compression, `-c1`
PalmDOC compression, and `-c2` Huffman/dictionary compression; `-verbose` enables
detailed diagnostics. `-o` takes a basename, not a destination path: the output is
written beside the input EPUB/OPF/HTML. The result is a combo MOBI; use KindleUnpack's
`-s` option to extract its standalone AZW3 portion for comparison.

KindleGen's exit status is preserved: `0` means success, `1` warnings, and `2` a
compiler error. Inspect the complete log when warnings occur. Compare books with
matching content and settings in the same reader; different compiler defaults can
produce different results without indicating a converter defect.

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

The PDF wrapper stops on a failed command, including qpdf warnings that produce
a nonzero exit status, or a missing required metadata stream. Do not interpret
printed metadata alone as a successful structural check.

## Investigation Workflow

1. Reproduce with `fbc -d` and retain the exact command, converter version, and configuration.
2. Open `MANIFEST` and `final.log`; find the book directory and published result.
3. Compare `book.pristine.txt` and `book.prepared.txt` when content, IDs, links, images, or footnotes differ
   from the source expectations.
4. For styling/layout, inspect KFX CSS/style traces and focused dumps, or PDF style/layout/
   glyph/font JSON. Compare KPF/KFX focused reports when using Previewer as a reference.
5. Run the corresponding external validator, then check reading/navigation in the target viewer.
6. For compressed KFX, use `kfxdump --extract-inner` and compare the uncompressed result
   to distinguish compression problems from book-generation problems.

When reporting an issue, include the command line, converter version, relevant configuration,
the complete debug report ZIP file (`fbc-report.zip` by default), validator output, and the
reader/version used to reproduce it. Attach the full ZIP archive rather than selected files
or excerpts from it.
