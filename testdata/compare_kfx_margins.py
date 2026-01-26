#!/usr/bin/env python3
"""
Compare KFX margin files between generated and reference outputs.
Matches storylines by content and reports vertical margin differences.
"""

import re
import sys
from dataclasses import dataclass
from typing import Optional


@dataclass
class MarginEntry:
    index: str
    item_type: str
    text: str
    mt: Optional[str] = None  # margin-top
    mb: Optional[str] = None  # margin-bottom
    ml: Optional[str] = None  # margin-left (for reference only, not compared)


def parse_margins(line: str) -> tuple[Optional[str], Optional[str], Optional[str]]:
    """Extract mt, mb, ml values from a line."""
    mt = mb = ml = None
    mt_match = re.search(r"mt=([0-9.]+lh)", line)
    mb_match = re.search(r"mb=([0-9.]+lh)", line)
    ml_match = re.search(r"ml=([0-9.]+%|\$\d+)", line)
    if mt_match:
        mt = mt_match.group(1)
    if mb_match:
        mb = mb_match.group(1)
    if ml_match:
        ml = ml_match.group(1)
    return mt, mb, ml


def parse_file(filepath: str) -> dict[str, list[MarginEntry]]:
    """Parse a margin file into storylines with entries."""
    storylines = {}
    current_storyline = None

    with open(filepath, "r", encoding="utf-8") as f:
        for line in f:
            line = line.rstrip()

            # Check for storyline header
            storyline_match = re.match(r"^storyline: (\w+)", line)
            if storyline_match:
                current_storyline = storyline_match.group(1)
                storylines[current_storyline] = []
                continue

            if current_storyline is None:
                continue

            # Parse entry lines like "  [0] container (3 items) (mt=1.66667lh, mb=0.833333lh)"
            # or "  [0.1] text "Some text..." (mt=0.55275lh)"
            entry_match = re.match(
                r"^\s+\[([0-9.]+)\]\s+(text|image|container)\s*(.*)$", line
            )
            if entry_match:
                index = entry_match.group(1)
                item_type = entry_match.group(2)
                rest = entry_match.group(3)

                # Extract text preview if present
                text_match = re.search(r'"([^"]*)"', rest)
                text = text_match.group(1) if text_match else ""

                mt, mb, ml = parse_margins(rest)

                entry = MarginEntry(
                    index=index,
                    item_type=item_type,
                    text=text[:30] if text else "",
                    mt=mt,
                    mb=mb,
                    ml=ml,
                )
                storylines[current_storyline].append(entry)

    return storylines


def match_storylines(
    ours: dict, ref: dict
) -> list[tuple[str, str, list[MarginEntry], list[MarginEntry]]]:
    """Match storylines between our output and reference by content similarity."""
    matches = []
    used_ref = set()

    for our_id, our_entries in ours.items():
        if not our_entries:
            continue

        # Build a content signature for our storyline
        our_sig = []
        for e in our_entries[:5]:  # Use first 5 entries for matching
            our_sig.append((e.item_type, e.text[:20] if e.text else ""))

        best_match = None
        best_score = 0

        for ref_id, ref_entries in ref.items():
            if ref_id in used_ref or not ref_entries:
                continue

            # Build reference signature
            ref_sig = []
            for e in ref_entries[:5]:
                ref_sig.append((e.item_type, e.text[:20] if e.text else ""))

            # Calculate similarity score
            score = 0
            for i, (our_item, ref_item) in enumerate(zip(our_sig, ref_sig)):
                if our_item[0] == ref_item[0]:  # Same type
                    score += 1
                    if our_item[1] == ref_item[1]:  # Same text
                        score += 2
                    elif (
                        our_item[1]
                        and ref_item[1]
                        and our_item[1][:10] == ref_item[1][:10]
                    ):
                        score += 1

            if score > best_score:
                best_score = score
                best_match = ref_id

        if best_match and best_score >= 3:  # Threshold for matching
            used_ref.add(best_match)
            matches.append((our_id, best_match, our_entries, ref[best_match]))

    return matches


def compare_entries(
    our_entries: list[MarginEntry],
    ref_entries: list[MarginEntry],
    our_id: str,
    ref_id: str,
) -> list[str]:
    """Compare entries between matched storylines and return differences."""
    diffs = []

    # Build lookup by index for reference
    ref_by_index = {e.index: e for e in ref_entries}
    our_by_index = {e.index: e for e in our_entries}

    all_indices = sorted(
        set(list(ref_by_index.keys()) + list(our_by_index.keys())),
        key=lambda x: [int(p) for p in x.split(".")],
    )

    for idx in all_indices:
        our_entry = our_by_index.get(idx)
        ref_entry = ref_by_index.get(idx)

        if our_entry and ref_entry:
            # Compare vertical margins (mt and mb)
            mt_diff = our_entry.mt != ref_entry.mt
            mb_diff = our_entry.mb != ref_entry.mb

            if mt_diff or mb_diff:
                text_preview = our_entry.text or ref_entry.text or ""
                if text_preview:
                    text_preview = f' "{text_preview[:25]}..."'

                diff_parts = []
                if mt_diff:
                    diff_parts.append(
                        f"mt: {our_entry.mt or 'none'} vs {ref_entry.mt or 'none'}"
                    )
                if mb_diff:
                    diff_parts.append(
                        f"mb: {our_entry.mb or 'none'} vs {ref_entry.mb or 'none'}"
                    )

                diffs.append(
                    f"  [{idx}] {our_entry.item_type}{text_preview}: {', '.join(diff_parts)}"
                )

    return diffs


def main():
    our_file = "/mnt/d/_Test-margins.txt"
    ref_file = "/mnt/d/test/_Test-kfxout-margins.txt"

    print("Parsing margin files...")
    our_storylines = parse_file(our_file)
    ref_storylines = parse_file(ref_file)

    print(f"Our file: {len(our_storylines)} storylines")
    print(f"Reference file: {len(ref_storylines)} storylines")
    print()

    print("Matching storylines by content...")
    matches = match_storylines(our_storylines, ref_storylines)
    print(f"Found {len(matches)} matched storyline pairs")
    print()

    print("=" * 80)
    print("VERTICAL MARGIN DIFFERENCES (mt=margin-top, mb=margin-bottom)")
    print("Format: ours vs reference")
    print("=" * 80)

    total_diffs = 0
    storylines_with_diffs = 0

    for our_id, ref_id, our_entries, ref_entries in matches:
        diffs = compare_entries(our_entries, ref_entries, our_id, ref_id)
        if diffs:
            storylines_with_diffs += 1
            total_diffs += len(diffs)

            # Find a representative text for the storyline
            rep_text = ""
            for e in our_entries:
                if e.text and len(e.text) > 5:
                    rep_text = e.text[:40]
                    break

            print(f"\nstoryline {our_id} <-> {ref_id}")
            if rep_text:
                print(f'  Content: "{rep_text}..."')
            print(f"  Differences ({len(diffs)}):")
            for diff in diffs:
                print(diff)

    print()
    print("=" * 80)
    print(f"SUMMARY: {total_diffs} differences in {storylines_with_diffs} storylines")
    print("=" * 80)

    # Also list unmatched storylines
    matched_ours = {m[0] for m in matches}
    matched_refs = {m[1] for m in matches}

    unmatched_ours = set(our_storylines.keys()) - matched_ours
    unmatched_refs = set(ref_storylines.keys()) - matched_refs

    if unmatched_ours:
        print(f"\nUnmatched our storylines: {sorted(unmatched_ours)}")
    if unmatched_refs:
        print(f"Unmatched ref storylines: {sorted(unmatched_refs)}")


if __name__ == "__main__":
    main()
