# MyHomeLib Integration

fb2cng includes `mhl-connector` for integration with MyHomeLib library management software on Windows.

## Installation Layout

MyHomeLib expects converter executables under its `converters` directory. The recommended layout is:

```text
MyHomeLib Installation Directory
├── MyHomeLib.exe
└── converters/
    ├── converter/
    │   ├── fbc.exe
    │   └── mhl-connector.exe
    ├── fb2epub/
    │   ├── fb2epub.exe       copy of, or symlink to, mhl-connector.exe
    │   └── fb2epub.yaml      optional fbc.exe configuration
    ├── fb2mobi/
    │   ├── fb2mobi.exe       copy of, or symlink to, mhl-connector.exe
    │   └── fb2mobi.yaml      optional fbc.exe configuration
    └── fb2pdf/
        ├── fb2pdf.cmd        required by MyHomeLib for PDF
        ├── fb2pdf.exe        copy of, or symlink to, mhl-connector.exe
        └── fb2pdf.yaml       optional fbc.exe configuration
```

If you copy `mhl-connector.exe` into each converter directory, keep `fbc.exe` in `converters/converter/` as shown above or make sure `fbc.exe` is available on `PATH`.

If you use symlinks, `mhl-connector.exe` should be located next to `fbc.exe`; the symlinked converter executables can point to it and no `PATH` change is required.

For PDF conversion, MyHomeLib expects `fb2pdf.cmd` rather than `fb2pdf.exe`. Create it next to `fb2pdf.exe` with this content:

```bat
@echo off
setlocal
set "EXE=%~dpn0.exe"
if not exist "%EXE%" (
    echo Executable not found: "%EXE%" 1>&2
    exit /b 1
)
"%EXE%" %*
exit /b %ERRORLEVEL%
```

## Connector Configuration

Since passing extra arguments through MyHomeLib is inconvenient, the connector
supports an optional `connector.yaml` in the invoked converter directory, next to
`fb2epub.exe`, `fb2mobi.exe`, or `fb2pdf.exe`.

This is **not** the main converter configuration: `connector.yaml` uses the small
schema below, while `fb2epub.yaml`, `fb2mobi.yaml`, and `fb2pdf.yaml` use the
normal [fbc configuration](config.md). Unknown fields are rejected.

Example:

```yaml
version: 1

# Redirect connector logs to a file.
# log_destination: connector.log

# Pass debug flag to fbc.
debug: false

# Output format override.
# output_format: epub3

# Explicitly mark Kindle output as ebook/EBOK.
kindle_ebook: false
```

## Behavior

The connector translates MyHomeLib converter calls into `fbc convert` calls and chooses output format based on the executable name or connector configuration.

| Invoked name | Default output | Configurable alternatives |
|---|---|---|
| `fb2epub.exe` | EPUB2 | `epub3`, `kepub`, `kepub3` |
| `fb2mobi.exe` | KFX | `azw8` |
| `fb2pdf.exe` | PDF | Use the default; no override is needed |

Despite the historical `fb2mobi.exe` name, its output is KFX/AZW8, not MOBI7 or a
combo MOBI. The current connector configuration validator accepts EPUB-family
and KFX/AZW8 overrides only; native AZW3 is available through `fbc convert --to azw3`
but not through `connector.yaml`. `kindle_ebook: true` passes `--ebook` for the
Kindle target.

The connector passes the destination **directory**, not MyHomeLib's requested
filename, to `fbc`; output naming therefore follows the main converter
configuration. It enables overwrite and limits a conversion to two minutes.

Use `debug: true` in `connector.yaml` when diagnosing MyHomeLib integration problems.
See [Debugging and validation](debugging.md) for converter report contents and inspection tools.
