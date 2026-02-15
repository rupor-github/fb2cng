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
import sys
from typing import List, Optional, Set


def _candidate_kfxinput_paths(cli_path: Optional[str]) -> List[Path]:
    paths: List[Path] = []

    if cli_path:
        paths.append(Path(cli_path))

    env_path = os.environ.get("KFXINPUT_PATH")
    if env_path:
        paths.append(Path(env_path))

    # Default checkout location in this workspace layout.
    # When run from this repo root (.../fb2cng), KFXInput is three levels up:
    #   ../../../KFXInput
    project_dir = Path.cwd().resolve()
    if len(project_dir.parents) >= 3:
        paths.append(project_dir.parents[2] / "KFXInput")

    # Common local checkouts.
    paths.extend(
        [
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


def main():
    parser = argparse.ArgumentParser(
        description="Process a KFX file with kfxlib and print warnings/errors with debug logging."
    )
    parser.add_argument("kfx_file", help="Path to .kfx file")
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
    except ImportError:
        last_err: Optional[Exception] = None
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

        if last_err is not None:
            print("ERROR: Failed to import kfxlib.", file=sys.stderr)
            print(
                "Install kfxlib into your environment, or point this script to a KFXInput checkout via:",
                file=sys.stderr,
            )
            print("  - --kfxinput /path/to/KFXInput", file=sys.stderr)
            print("  - KFXINPUT_PATH=/path/to/KFXInput", file=sys.stderr)
            if args.kfxinput:
                print(f"Tried --kfxinput={args.kfxinput}", file=sys.stderr)
            else:
                print(
                    "Tried common locations (including ~/projects/KFXInput).",
                    file=sys.stderr,
                )
            print(f"ImportError: {last_err}", file=sys.stderr)
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

    try:
        # Create YJ_Book instance and decode it
        book = YJ_Book(kfx_file)
        book.decode_book(retain_yj_locals=True)

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
        # Clean up logger
        if set_logger is not None:
            set_logger()


if __name__ == "__main__":
    main()
