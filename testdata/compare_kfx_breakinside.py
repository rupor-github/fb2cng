#!/usr/bin/env python3
"""Compare break-inside wrapper placement between two kfxdump storyline files.

The raw kfxdump output uses different storyline IDs (l1 vs l3S) and different
style IDs (s32 vs s4J) between builds. This script compares the *placement* of
"break-inside: avoid" by extracting a stable content signature: the first text
snippet found inside each wrapper that has break-inside.

Usage:
  compare_kfx_breakinside.py <ref-storyline.txt> <our-storyline.txt>

Exit code:
  0 if placements match, 1 otherwise.
"""

from __future__ import annotations

import re
import sys
from dataclasses import dataclass
from typing import Iterable


@dataclass(frozen=True)
class BreakInsideOccurrence:
    storyline: str
    eid: int | None
    style: str
    snippet: str
    line_no: int


RE_STORYLINE = re.compile(r"^\s*id=\$\d+ \((l[0-9A-Z]+)\)\s*$")
RE_EID = re.compile(r"\bid \(\$155\): int\((\d+)\)")
RE_STYLE_BREAKINSIDE = re.compile(
    r"\bstyle \(\$157\): symbol\(\"(s[^\"]+)\"\)\s*/\*[^*]*\bbreak-inside: avoid\b"
)
RE_CONTENT_INDEX = re.compile(r"\bindex \(\$403\): \d+\s*/\* \"(.*?)\" \*/")


def _normalize_snippet(s: str) -> str:
    s = s.strip()
    s = re.sub(r"\s+", " ", s)
    return s


def _iter_lines(path: str) -> Iterable[tuple[int, str]]:
    with open(path, "r", encoding="utf-8") as f:
        for i, line in enumerate(f, 1):
            yield i, line.rstrip("\n")


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
        # Prefer backward when equally distant (content usually appears before wrapper style).
        tie_break = 0 if j <= i else 1
        score = dist * 10 + tie_break
        text = _normalize_snippet(m.group(1))
        if best_score is None or score < best_score:
            best_score = score
            best_text = text

    return best_text


def extract_breakinside_occurrences(
    path: str, window: int = 120
) -> list[BreakInsideOccurrence]:
    lines = [line for _, line in _iter_lines(path)]
    # Track storyline by scanning up to the current line.
    storyline_at = [""] * len(lines)
    cur = ""
    for i, line in enumerate(lines):
        m = RE_STORYLINE.match(line)
        if m:
            cur = m.group(1)
        storyline_at[i] = cur

    out: list[BreakInsideOccurrence] = []
    for i, line in enumerate(lines):
        m = RE_STYLE_BREAKINSIDE.search(line)
        if not m:
            continue

        style = m.group(1)
        story = storyline_at[i] or "?"

        # Find the closest EID near the break-inside style line.
        eid: int | None = None
        for j in range(i, max(-1, i - 20), -1):
            me = RE_EID.search(lines[j])
            if me:
                eid = int(me.group(1))
                break

        snippet = _closest_content_snippet(lines, i, window)

        out.append(
            BreakInsideOccurrence(
                storyline=story,
                eid=eid,
                style=style,
                snippet=snippet,
                line_no=i + 1,
            )
        )

    return out


def main() -> int:
    if len(sys.argv) != 3:
        print(
            "Usage: compare_kfx_breakinside.py <ref-storyline.txt> <our-storyline.txt>"
        )
        return 2

    ref_path = sys.argv[1]
    our_path = sys.argv[2]

    ref = extract_breakinside_occurrences(ref_path)
    ours = extract_breakinside_occurrences(our_path)

    def to_multiset(items: list[BreakInsideOccurrence]) -> dict[str, int]:
        out: dict[str, int] = {}
        for it in items:
            key = it.snippet or "<no-snippet>"
            out[key] = out.get(key, 0) + 1
        return out

    ref_ms = to_multiset(ref)
    our_ms = to_multiset(ours)

    if ref_ms == our_ms:
        # Order differences are expected when storyline splitting differs.
        if [o.snippet for o in ref] == [o.snippet for o in ours]:
            print(f"OK: break-inside placement matches ({len(ours)} occurrence(s))")
        else:
            print(
                f"OK: break-inside placement matches as multiset ({len(ours)} occurrence(s)); order differs"
            )
        return 0

    print("MISMATCH: break-inside placement differs")
    print(f"ref occurrences: {len(ref)}")
    print(f"our occurrences: {len(ours)}")

    missing = []
    extra = []
    for k in sorted(set(ref_ms) | set(our_ms)):
        rc = ref_ms.get(k, 0)
        oc = our_ms.get(k, 0)
        if oc < rc:
            missing.append((k, rc - oc))
        elif oc > rc:
            extra.append((k, oc - rc))

    if missing:
        print("missing in our output:")
        for k, n in missing[:10]:
            print(f"  {n}x {k!r}")
        if len(missing) > 10:
            print(f"  ... ({len(missing) - 10} more)")
    if extra:
        print("extra in our output:")
        for k, n in extra[:10]:
            print(f"  {n}x {k!r}")
        if len(extra) > 10:
            print(f"  ... ({len(extra) - 10} more)")

    probe = (missing[0][0] if missing else None) or (extra[0][0] if extra else None)
    if probe:

        def find_one(
            items: list[BreakInsideOccurrence], key: str
        ) -> BreakInsideOccurrence | None:
            for it in items:
                if (it.snippet or "<no-snippet>") == key:
                    return it
            return None

        rr = find_one(ref, probe)
        oo = find_one(ours, probe)
        print("example:")
        if rr:
            print(
                f"  ref: story={rr.storyline} eid={rr.eid} style={rr.style} line={rr.line_no} snippet={rr.snippet!r}"
            )
        if oo:
            print(
                f"  our: story={oo.storyline} eid={oo.eid} style={oo.style} line={oo.line_no} snippet={oo.snippet!r}"
            )

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
