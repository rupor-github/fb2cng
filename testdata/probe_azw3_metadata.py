#!/usr/bin/env python3
"""Drop selected EXTH fields for native AZW3 device diagnostics, without changing content."""

import argparse
import hashlib
import json
from pathlib import Path
import struct

from compare_azw3 import inspect, inspect_kindle_metadata, inspect_raw_breaks, palm_records, unpack


def drop_exth_fields(source, fields):
    records = palm_records(source)
    original = source.read_bytes()
    header = records[0]
    if len(header) < 280 or header[16:20] != b"MOBI" or struct.unpack_from(">H", header, 12)[0] != 0:
        raise RuntimeError("metadata probes require an unencrypted native KF8 header")
    exth = 16 + struct.unpack_from(">I", header, 20)[0]
    if exth + 12 > len(header) or header[exth:exth + 4] != b"EXTH":
        raise RuntimeError("missing EXTH header")
    length, count = struct.unpack_from(">II", header, exth + 4)
    old_end = exth + ((length + 3) // 4) * 4
    if length < 12 or old_end > len(header):
        raise RuntimeError("invalid EXTH length")
    retained, removed, at = [], set(), exth + 12
    for _ in range(count):
        if at + 8 > exth + length:
            raise RuntimeError("truncated EXTH record")
        key, size = struct.unpack_from(">II", header, at)
        if size < 8 or at + size > exth + length:
            raise RuntimeError("invalid EXTH record size")
        if key in fields:
            removed.add(key)
        else:
            retained.append(header[at:at + size])
        at += size
    if at != exth + length or removed != set(fields):
        raise RuntimeError("requested fields absent or EXTH record lengths inconsistent")
    payload = b"".join(retained)
    new_exth = b"EXTH" + struct.pack(">II", 12 + len(payload), len(retained)) + payload
    new_exth += b"\0" * (-len(new_exth) % 4)
    new_header = bytearray(header[:exth] + new_exth + header[old_end:])
    delta = len(new_header) - len(header)
    title_offset = struct.unpack_from(">I", header, 84)[0]
    if title_offset < old_end:
        raise RuntimeError("unsupported title position before EXTH end")
    struct.pack_into(">I", new_header, 84, title_offset + delta)
    first_record = struct.unpack_from(">I", original, 78)[0]
    prefix = bytearray(original[:first_record])
    for i in range(1, len(records)):
        offset = struct.unpack_from(">I", prefix, 78 + i * 8)[0]
        struct.pack_into(">I", prefix, 78 + i * 8, offset + delta)
    return bytes(prefix) + bytes(new_header) + b"".join(records[1:])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path, help="new diagnostic directory")
    parser.add_argument("--drop-exth", required=True, help="comma-separated EXTH record types")
    args = parser.parse_args()
    fields = {int(value) for value in args.drop_exth.split(",")}
    source, output = args.source.resolve(), args.output.resolve()
    original_metadata = inspect_kindle_metadata(source)
    data = drop_exth_fields(source, fields)
    output.mkdir(parents=True, exist_ok=False)
    book = output / "book.azw3"
    book.write_bytes(data)
    if palm_records(book)[1:] != palm_records(source)[1:]:
        raise RuntimeError("metadata control changed a non-header record")
    decoded = output / "unpacked"
    unpack(book, decoded, output / "unpack.log")
    inspection = inspect(decoded)
    inspection["raw_pagination"] = inspect_raw_breaks(book, decoded)
    inspection["kindle_metadata"] = inspect_kindle_metadata(book)
    if inspection["broken_links"]:
        raise RuntimeError("metadata control has broken local links")
    (output / "inspection.json").write_text(json.dumps(inspection, ensure_ascii=False, indent=2) + "\n")
    manifest = {"source": str(source), "source_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
                "control_sha256": hashlib.sha256(data).hexdigest(), "dropped_exth": sorted(fields),
                "all_nonheader_records_identical": True, "original_metadata": original_metadata,
                "control_metadata": inspection["kindle_metadata"]}
    (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    print(book)


if __name__ == "__main__":
    main()
