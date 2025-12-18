#!/usr/bin/env python3
# vim:fileencoding=UTF-8:ts=4:sw=4:sta:et:sts=4:ai

"""
KFX file processor using kfxlib from KFXInput Calibre plugin.
Processes a KFX file and prints all warnings and errors with debug logging enabled.
"""

import logging
import sys
import os

# Add KFXInput to the Python path
sys.path.insert(0, os.path.expanduser('~/projects/KFXInput'))

try:
    from kfxlib import YJ_Book, JobLog, set_logger
except ImportError as e:
    print(f"ERROR: Failed to import kfxlib from ~/projects/KFXInput: {e}", file=sys.stderr)
    print("Please ensure the KFXInput project is located at ~/projects/KFXInput", file=sys.stderr)
    sys.exit(1)


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
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <kfx_file>", file=sys.stderr)
        print("", file=sys.stderr)
        print("Process a KFX file and display all warnings, errors, and debug information.", file=sys.stderr)
        sys.exit(1)
    
    kfx_file = sys.argv[1]
    
    if not os.path.exists(kfx_file):
        print(f"ERROR: File not found: {kfx_file}", file=sys.stderr)
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
        set_logger()


if __name__ == '__main__':
    main()
