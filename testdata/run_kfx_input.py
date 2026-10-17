#!/usr/bin/env python3
# vim:fileencoding=UTF-8:ts=4:sw=4:sta:et:sts=4:ai

"""
KFX file processor using kfxlib from KFXInput Calibre plugin.
Processes a KFX file and prints all warnings and errors with debug logging enabled.
"""

import argparse
import logging
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from typing import List, Optional, Set


def _candidate_kfxinput_paths(cli_path: Optional[str]) -> List[Path]:
    paths: List[Path] = []

    if cli_path:
        paths.append(Path(cli_path))

    env_path = os.environ.get("KFXINPUT_PATH")
    if env_path:
        paths.append(Path(env_path))

    book_tests_path = os.environ.get("BOOK_TESTS_PATH")
    if book_tests_path:
        paths.append(Path(book_tests_path) / "KFXInput")

    # Default checkout location in this workspace layout.
    # When run from this repo root (.../fb2cng), book-tests is three levels up:
    #   ../../../book-tests/KFXInput
    project_dir = Path.cwd().resolve()
    if len(project_dir.parents) >= 3:
        paths.append(project_dir.parents[2] / "book-tests" / "KFXInput")
        paths.append(project_dir.parents[2] / "KFXInput")

    # Common local checkouts.
    paths.extend(
        [
            Path.home() / "projects" / "book-tests" / "KFXInput",
            Path.home() / "projects" / "KFXInput",
            Path.home() / "src" / "KFXInput",
        ]
    )

    # If the plugin repo is checked out next to this repo.
    here = Path(__file__).resolve()
    for p in here.parents:
        paths.append(p / "KFXInput")

    # De-dup while preserving order.
    out: List[Path] = []
    seen: Set[Path] = set()
    for p in paths:
        p = p.expanduser()
        if p in seen:
            continue
        seen.add(p)
        out.append(p)
    return out


def _try_import_kfxlib(extra_path: Optional[Path]):
    if extra_path is not None:
        sys.path.insert(0, str(extra_path))
    from kfxlib import JobLog, YJ_Book, set_logger  # type: ignore

    return YJ_Book, JobLog, set_logger


def _print_kfxinput_install_hint(args_kfxinput: Optional[str], last_err: Optional[Exception]) -> None:
    print("ERROR: Failed to import kfxlib from KFXInput.", file=sys.stderr)
    print(file=sys.stderr)
    print("Install or unpack the KFXInput Calibre plugin checkout here:", file=sys.stderr)
    print("  /home/rupor/projects/book-tests/KFXInput", file=sys.stderr)
    print(file=sys.stderr)
    print("Or point this script to it with one of:", file=sys.stderr)
    print("  --kfxinput /path/to/KFXInput", file=sys.stderr)
    print("  KFXINPUT_PATH=/path/to/KFXInput", file=sys.stderr)
    print("  BOOK_TESTS_PATH=/path/to/book-tests", file=sys.stderr)
    print(file=sys.stderr)
    print("Expected directory must contain: kfxlib/", file=sys.stderr)
    print(
        "Project source: https://www.mobileread.com/forums/showthread.php?t=291290&highlight=KFXInput",
        file=sys.stderr,
    )
    if args_kfxinput:
        print(f"Tried --kfxinput={args_kfxinput}", file=sys.stderr)
    else:
        print("Tried common locations including ~/projects/book-tests/KFXInput.", file=sys.stderr)
    if last_err is not None:
        print(f"ImportError: {last_err}", file=sys.stderr)


class ConsoleLogger:
    """Simple logger that outputs to console with DEBUG level support."""

    def __init__(self, level=logging.DEBUG):
        self.level = level

    def debug(self, msg):
        if self.level <= logging.DEBUG:
            print(f"DEBUG: {msg}")

    def info(self, msg):
        if self.level <= logging.INFO:
            print(f"INFO: {msg}")

    def warn(self, msg):
        if self.level <= logging.WARNING:
            print(f"WARNING: {msg}")

    def warning(self, msg):
        self.warn(msg)

    def error(self, msg):
        if self.level <= logging.ERROR:
            print(f"ERROR: {msg}")

    def exception(self, msg):
        print(f"EXCEPTION: {msg}")
        import traceback

        traceback.print_exc()


def unwrap_plaintext_envelope(source: str, destination: Path, cli_tool: Optional[str]) -> str:
    with open(source, "rb") as book:
        if book.read(8) != b"\xeaDRMION\xee":
            return source
    selected = cli_tool or os.environ.get("KFXCOMPRESS_PATH")
    if not selected:
        local = Path(__file__).resolve().parents[1] / "build" / "kfxcompress"
        selected = str(local) if local.is_file() else shutil.which("kfxcompress")
    if not selected:
        raise RuntimeError(
            "Plaintext KFX envelopes require kfxcompress. Run 'go tool task debug-tools', "
            "or supply --kfxcompress PATH / KFXCOMPRESS_PATH."
        )
    subprocess.run([selected, "uncompress", source, str(destination)], check=True)
    print(f"Unwrapped plaintext envelope for KFXInput: {source}")
    return str(destination)


def check_native_kvg(book, logger) -> None:
    """Catch native-wire errors that KFXInput's permissive SVG decoder accepts.

    Amazon's SVG adapter writes fixed_width/fixed_height as scalar integers.
    They are not CSS dimensions ({value, unit}), even though KFXInput can
    interpret those structs while reconstructing SVG. KPV also rejects Ion
    floats in shape data: paths, stroke widths, opacity and transforms must
    use Ion decimals. This is a wire check,
    not a substitute for testing pagination/rendering in Kindle Previewer.
    """
    from kfxlib.ion import IonFloat, IonInt, ion_type

    def first_float_path(value, path):
        if ion_type(value) is IonFloat:
            return path
        if isinstance(value, dict):
            for field, child in value.items():
                found = first_float_path(child, f"{path}.{field}")
                if found is not None:
                    return found
        elif isinstance(value, list):
            for index, child in enumerate(value):
                found = first_float_path(child, f"{path}[{index}]")
                if found is not None:
                    return found
        return None

    def visit(value):
        if isinstance(value, dict):
            if value.get("$159") == "$272":
                eid = value.get("$155", "unknown")
                for field, name in (("$66", "fixed_width"), ("$67", "fixed_height")):
                    # A child KVG can inherit a viewport from its parent.
                    if field not in value:
                        continue
                    dimension = value.get(field)
                    if ion_type(dimension) is not IonInt or dimension <= 0:
                        logger.error(
                            f"Native KVG eid={eid}: {name} ({field}) must be a positive scalar Ion integer, "
                            f"got {type(dimension).__name__}: {dimension!r}"
                        )
                float_path = first_float_path(value.get("$250", []), "$250")
                if float_path is not None:
                    logger.error(
                        f"Native KVG eid={eid}: shape data at {float_path} uses Ion float; "
                        "Kindle Previewer requires Ion decimals for shape numbers"
                    )
            for child in value.values():
                visit(child)
        elif isinstance(value, list):
            for child in value:
                visit(child)

    for fragment in book.fragments:
        visit(fragment.value)


def main():
    parser = argparse.ArgumentParser(
        description="Process a KFX file with kfxlib and print warnings/errors with debug logging."
    )
    parser.add_argument("kfx_file", help="Path to .kfx file")
    parser.add_argument("--kfxcompress", metavar="PATH", help="Path to kfxcompress for plaintext-envelope unwrapping")
    parser.add_argument(
        "--kfxinput",
        metavar="PATH",
        help=(
            "Path to a local KFXInput checkout (directory that contains kfxlib/). "
            "If omitted, uses $KFXINPUT_PATH or tries common locations."
        ),
    )
    args = parser.parse_args()

    kfx_file = args.kfx_file

    if not os.path.exists(kfx_file):
        print(f"ERROR: File not found: {kfx_file}", file=sys.stderr)
        sys.exit(1)

    # Import kfxlib either from the active environment or from a local KFXInput checkout.
    YJ_Book = JobLog = set_logger = None
    try:
        YJ_Book, JobLog, set_logger = _try_import_kfxlib(None)
    except ImportError as e:
        last_err: Optional[Exception] = e
        loaded_from: Optional[Path] = None
        for candidate in _candidate_kfxinput_paths(args.kfxinput):
            # Skip paths that don't look like a KFXInput checkout.
            if not (candidate / "kfxlib").is_dir():
                continue
            try:
                YJ_Book, JobLog, set_logger = _try_import_kfxlib(candidate)
                loaded_from = candidate
                last_err = None
                break
            except ImportError as e:
                last_err = e
                continue

        if YJ_Book is None or JobLog is None or set_logger is None:
            _print_kfxinput_install_hint(args.kfxinput, last_err)
            sys.exit(1)

        if loaded_from is not None:
            print(f"Loaded kfxlib from: {loaded_from}")

    if YJ_Book is None or JobLog is None or set_logger is None:
        print("ERROR: kfxlib import unexpectedly failed.", file=sys.stderr)
        sys.exit(1)

    # Create console logger with DEBUG level
    console_logger = ConsoleLogger(level=logging.DEBUG)

    # Create JobLog wrapper to collect errors and warnings
    job_log = JobLog(console_logger)

    # Set the logger for kfxlib
    set_logger(job_log)

    print(f"Processing KFX file: {kfx_file}")
    print("=" * 80)

    temporary = tempfile.TemporaryDirectory(prefix="fbc-kfxinput-")
    try:
        kfx_file = unwrap_plaintext_envelope(kfx_file, Path(temporary.name) / "inner.kfx", args.kfxcompress)
        # Create YJ_Book instance and decode it
        book = YJ_Book(kfx_file)
        book.decode_book(retain_yj_locals=True)
        check_native_kvg(book, job_log)

        print("=" * 80)
        print("Processing complete!")
        print()

        # Print summary
        if job_log.warnings:
            print(f"\n{len(job_log.warnings)} WARNING(S) found:")
            print("-" * 80)
            for i, warning in enumerate(job_log.warnings, 1):
                print(f"{i}. {warning}")
        else:
            print("\nNo warnings found.")

        if job_log.errors:
            print(f"\n{len(job_log.errors)} ERROR(S) found:")
            print("-" * 80)
            for i, error in enumerate(job_log.errors, 1):
                print(f"{i}. {error}")
        else:
            print("\nNo errors found.")

        # Exit with error code if there were errors
        if job_log.errors:
            sys.exit(1)

    except Exception as e:
        print("=" * 80)
        print(f"\nFATAL ERROR: {e}", file=sys.stderr)
        import traceback

        traceback.print_exc()

        # Print collected warnings and errors before fatal error
        if job_log.warnings:
            print(f"\nWarnings before fatal error ({len(job_log.warnings)}):")
            for warning in job_log.warnings:
                print(f"  - {warning}")

        if job_log.errors:
            print(f"\nErrors before fatal error ({len(job_log.errors)}):")
            for error in job_log.errors:
                print(f"  - {error}")

        sys.exit(1)

    finally:
        temporary.cleanup()
        # Clean up logger
        if set_logger is not None:
            set_logger()


if __name__ == "__main__":
    main()
