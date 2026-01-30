#!/usr/bin/env python3
"""Compare yj-break-* placement between two kfxdump storyline files.

KP3 dumps use different storyline IDs and style IDs across builds.
This script compares the *placement* of yj-break-before/after: avoid using a
stable content signature:

- For image entries: the resource path in the resource_name comment
  (e.g. resource/rsrc1B3, 2048x170).
- For text entries: a nearby content index string ("...") similar to
  compare_kfx_breakinside.py.

Usage:
  compare_kfx_yjbreak.py <ref-storyline.txt> <our-storyline.txt>

Exit code:
  0 if placements match (as multiset), 1 otherwise.
"""

from __future__ import annotations

import re
import sys
from collections import Counter
from dataclasses import dataclass
from typing import Iterable


@dataclass(frozen=True)
class YjBreakOccurrence:
    prop: str  # "before" | "after"
    storyline: str
    eid: int | None
    style: str
    kind: str  # "text" | "image" | "unknown"
    ident: str
    line_no: int


RE_STORYLINE = re.compile(r"^\s*id=\$\d+ \((l[0-9A-Z]+)\)\s*$")
RE_EID = re.compile(r"\bid \(\$155\): int\((\d+)\)")
RE_STYLE = re.compile(r"\bstyle \(\$157\): symbol\(\"(s[^\"]+)\"\)\s*/\*([^*]*)\*/")
RE_CONTENT_INDEX = re.compile(r"\bindex \(\$403\): \d+\s*/\* \"(.*?)\" \*/")
RE_RESOURCE_LINE = re.compile(
    r"\bresource_name \(\$175\): symbol\(\"(e[^\"]+)\"\)\s*/\*\s*(resource/[^,]+)(?:,\s*([^*]+?))?\s*\*/"
)


def _iter_lines(path: str) -> Iterable[tuple[int, str]]:
    with open(path, "r", encoding="utf-8") as f:
        for i, line in enumerate(f, 1):
            yield i, line.rstrip("\n")


def _normalize(s: str) -> str:
    s = s.strip()
    s = re.sub(r"\s+", " ", s)
    return s


def _closest_content_snippet(lines: list[str], i: int, window: int) -> str:
    best_score = None
    best_text = ""

    lo = max(0, i - window)
    hi = min(len(lines), i + window + 1)
    for j in range(lo, hi):
        m = RE_CONTENT_INDEX.search(lines[j])
        if not m:
            continue
        dist = abs(j - i)
        tie_break = 0 if j <= i else 1
        score = dist * 10 + tie_break
        text = _normalize(m.group(1))
        if best_score is None or score < best_score:
            best_score = score
            best_text = text
    return best_text


def _forward_resource_ident(lines: list[str], i: int, window: int) -> str:
    hi = min(len(lines), i + window + 1)
    for j in range(i, hi):
        m = RE_RESOURCE_LINE.search(lines[j])
        if not m:
            continue
        # Prefer stable comment info. Include dims if present.
        path = m.group(2).strip()
        extra = (m.group(3) or "").strip()
        if extra:
            return f"{path}, {extra}"
        return path
    return ""


def extract_occurrences(path: str) -> list[YjBreakOccurrence]:
    lines = [line for _, line in _iter_lines(path)]

    storyline_at = [""] * len(lines)
    cur = ""
    for i, line in enumerate(lines):
        m = RE_STORYLINE.match(line)
        if m:
            cur = m.group(1)
        storyline_at[i] = cur

    out: list[YjBreakOccurrence] = []
    for i, line in enumerate(lines):
        m = RE_STYLE.search(line)
        if not m:
            continue

        style = m.group(1)
        comment = m.group(2)

        prop = None
        if "yj-break-before: avoid" in comment:
            prop = "before"
        elif "yj-break-after: avoid" in comment:
            prop = "after"
        else:
            continue

        story = storyline_at[i] or "?"
        eid: int | None = None
        for j in range(i, max(-1, i - 31), -1):
            me = RE_EID.search(lines[j])
            if me:
                eid = int(me.group(1))
                break

        # Prefer resource ident when present (images).
        ident = _forward_resource_ident(lines, i, window=20)
        if ident:
            kind = "image"
        else:
            kind = "text"
            ident = _closest_content_snippet(lines, i, window=200) or "<no-snippet>"

        out.append(
            YjBreakOccurrence(
                prop=prop,
                storyline=story,
                eid=eid,
                style=style,
                kind=kind,
                ident=ident,
                line_no=i + 1,
            )
        )

    return out


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: compare_kfx_yjbreak.py <ref-storyline.txt> <our-storyline.txt>")
        return 2

    ref_path = sys.argv[1]
    our_path = sys.argv[2]

    ref = extract_occurrences(ref_path)
    ours = extract_occurrences(our_path)

    def key(o: YjBreakOccurrence) -> tuple[str, str, str]:
        return (o.prop, o.kind, o.ident)

    ref_ms = Counter(key(o) for o in ref)
    our_ms = Counter(key(o) for o in ours)

    if ref_ms == our_ms:
        if [key(o) for o in ref] == [key(o) for o in ours]:
            print(f"OK: yj-break placement matches ({len(ours)} occurrence(s))")
        else:
            print(
                f"OK: yj-break placement matches as multiset ({len(ours)} occurrence(s)); order differs"
            )
        return 0

    missing = ref_ms - our_ms
    extra = our_ms - ref_ms

    print("MISMATCH: yj-break placement differs")
    print(f"ref occurrences: {len(ref)}")
    print(f"our occurrences: {len(ours)}")
    print(f"missing keys: {sum(missing.values())} distinct={len(missing)}")
    print(f"extra keys: {sum(extra.values())} distinct={len(extra)}")

    if missing:
        print("missing in our output:")
        for (prop, kind, ident), n in missing.most_common(10):
            print(f"  {n}x {prop}/{kind}: {ident!r}")
    if extra:
        print("extra in our output:")
        for (prop, kind, ident), n in extra.most_common(10):
            print(f"  {n}x {prop}/{kind}: {ident!r}")

    # Provide one example location for the first missing/extra key.
    probe = None
    if missing:
        probe = next(iter(missing))
    elif extra:
        probe = next(iter(extra))
    if probe:

        def find_one(items: list[YjBreakOccurrence], k: tuple[str, str, str]):
            for it in items:
                if key(it) == k:
                    return it
            return None

        rr = find_one(ref, probe)
        oo = find_one(ours, probe)
        print("example:")
        if rr:
            print(
                f"  ref: {rr.prop}/{rr.kind} story={rr.storyline} eid={rr.eid} style={rr.style} line={rr.line_no} ident={rr.ident!r}"
            )
        if oo:
            print(
                f"  our: {oo.prop}/{oo.kind} story={oo.storyline} eid={oo.eid} style={oo.style} line={oo.line_no} ident={oo.ident!r}"
            )

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
