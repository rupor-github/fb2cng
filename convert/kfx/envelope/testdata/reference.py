"""Print independently generated synthetic reference envelopes as hexadecimal fixtures."""

from pathlib import Path
import sys

sys.path.insert(0, str(Path(sys.argv[1]).resolve()))
from prepare_indexed_plaintext_tests import indexed

for mode in ("plaintext", "lzma", "lzma64"):
    source = b"CONT\x00reference\xff\x80"
    if mode != "plaintext":
        source = "Привет, 世界!\x00".encode("utf8") * (3000 if mode == "lzma64" else 210) + source
    data, _, _ = indexed(source, 2, mode != "plaintext", False, "frame", True,
        page_size=65536 if mode == "lzma64" else 4096, algorithm=mode != "plaintext")
    print(mode, data.hex())
