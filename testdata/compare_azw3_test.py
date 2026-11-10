import json
import hashlib
from pathlib import Path
import sys
import struct
import tempfile
import unittest
import xml.etree.ElementTree as ET
import zipfile

from compare_azw3 import (audit_pagination_report, export_reference_publication, inspect, inspect_break_markup,
                         inspect_kindle_metadata, inspect_native_anchors, inspect_raw_breaks, inspect_resc_spine,
                         load_config, palm_records, run, self_closing_nonvoid)
from probe_azw3_metadata import drop_exth_fields


class AZW3ComparisonTests(unittest.TestCase):
    def test_metadata_probe_preserves_title_and_content_records(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "source.azw3"
            fields = [(113, b"B012345678"), (501, b"PDOC"), (504, b"B012345678"), (116, struct.pack(">I", 320))]
            payload = b"".join(struct.pack(">II", key, 8 + len(value)) + value for key, value in fields)
            exth = b"EXTH" + struct.pack(">II", 12 + len(payload), len(fields)) + payload
            exth += b"\0" * (-len(exth) % 4)
            header = bytearray(280)
            header[16:20] = b"MOBI"
            struct.pack_into(">I", header, 20, 264)
            title = "Title café".encode()
            struct.pack_into(">II", header, 84, 280 + len(exth), len(title))
            header.extend(exth + title + b"\0")
            pdb = bytearray(78)
            struct.pack_into(">H", pdb, 76, 3)
            table = struct.pack(">I4xI4xI4x", 102, 102 + len(header), 106 + len(header))
            original = bytes(pdb + table + header + b"TEXTIMAGE")
            source.write_bytes(original)
            for removed in ({113}, {116}, {113, 501, 504}):
                with self.subTest(removed=removed):
                    control = root / "control.azw3"
                    control.write_bytes(drop_exth_fields(source, removed))
                    records = palm_records(control)
                    self.assertEqual(records[1:], palm_records(source)[1:])
                    offset = struct.unpack_from(">I", records[0], 84)[0]
                    self.assertEqual(records[0][offset:offset + len(title)], title)
                    metadata = inspect_kindle_metadata(control)
                    self.assertEqual(metadata["asin"], [] if 113 in removed else ["B012345678"])
                    self.assertEqual(metadata["start_reading"], [] if 116 in removed else [320])
                    self.assertEqual(metadata["content_type"], [] if 501 in removed else ["PDOC"])
                    self.assertEqual(source.read_bytes(), original)
            with self.assertRaises(RuntimeError):
                drop_exth_fields(source, {999})

    def test_reading_order_distinguishes_real_and_virtual_covers(self):
        for xml, expected in [
            (b'<package><metadata/><spine><itemref idref="cover" skelid="0"/><itemref skelid="1"/></spine></package>',
             [{"idref": "cover", "skelid": "0", "linear": "yes"}, {"idref": None, "skelid": "1", "linear": "yes"}]),
            (b'<metadata/><spine><itemref idref="cover" linear="no"/><itemref skelid="0"/></spine>',
             [{"idref": "cover", "skelid": None, "linear": "no"}, {"idref": None, "skelid": "0", "linear": "yes"}]),
        ]:
            size = len(xml)
            digits = "0123456789ABCDEFGHIJKLMNOPQRSTUV"
            encoded = ""
            while size:
                encoded = digits[size % 32] + encoded
                size //= 32
            header = ("size=" + encoded + "&version=1&type=1").encode()
            record = b"RESC" + struct.pack(">III", 16, 1, len(header)) + header + xml
            self.assertEqual(inspect_resc_spine(record), expected)
            with self.assertRaises(RuntimeError):
                inspect_resc_spine(record[:-1])

    def test_cover_resources_and_document_classification(self):
        with tempfile.TemporaryDirectory() as directory:
            book = Path(directory) / "book.azw3"
            for content_type in (b"PDOC", b"EBOK", None):
                header = bytearray(280)
                header[16:20] = b"MOBI"
                struct.pack_into(">I", header, 20, 264)
                struct.pack_into(">I", header, 108, 1)
                fields = [(201, struct.pack(">I", 0)), (202, struct.pack(">I", 1)), (116, struct.pack(">I", 456))]
                if content_type:
                    fields.append((501, content_type))
                records = b"".join(struct.pack(">II", key, 8 + len(value)) + value for key, value in fields)
                header.extend(b"EXTH" + struct.pack(">II", 12 + len(records), len(fields)) + records)
                cover, thumbnail = b"\xff\xd8cover", b"\xff\xd8thumbnail"
                pdb = bytearray(78)
                pdb[:4] = b"e\xdf\xc6A"
                struct.pack_into(">H", pdb, 76, 3)
                table = struct.pack(">I4xI4xI4x", 102, 102 + len(header), 102 + len(header) + len(cover))
                book.write_bytes(pdb + table + header + cover + thumbnail)
                result = inspect_kindle_metadata(book)
                self.assertEqual(result["database_name"], "eßÆA")
                self.assertEqual(result["database_name_hex"], "65dfc641")
                self.assertEqual(result["content_type"], [content_type.decode()] if content_type else [])
                self.assertEqual(result["start_reading"], [456])
                self.assertEqual(result["cover"]["sha256"], hashlib.sha256(cover).hexdigest())
                self.assertEqual(result["thumbnail"]["sha256"], hashlib.sha256(thumbnail).hexdigest())

    def test_native_note_anchors_and_table_exclusion(self):
        result = inspect_native_anchors(b'''<body aid="0000"><a id="ref" aid="0001" epub:type="noteref">1</a>
<aside id="note" aid="0002" epub:type="footnote"><p aid="0003">Note</p></aside>
<table><tr><td><a id="table-ref" epub:type="noteref">2</a></td></tr></table></body>''')
        self.assertEqual(result["count"], 4)
        self.assertEqual(result["duplicates"], [])
        self.assertEqual(result["unanchored_notes"], [])
        self.assertEqual(result["table_notes"], [{"tag": "a", "id": "table-ref"}])
        result = inspect_native_anchors(b'''<body aid="0000"><a id="ref" epub:type="noteref">1</a>
<aside id="note" epub:type="footnote"><p aid="0000">Note</p></aside></body>''')
        self.assertEqual(result["duplicates"], ["0000"])
        self.assertEqual(result["unanchored_notes"], [{"tag": "a", "id": "ref"}, {"tag": "aside", "id": "note"}])

    def test_native_breaks_are_not_decoded_css_divs(self):
        result = inspect_break_markup(b'''<html><body><div data-AmznPageBreak="always"></div>
<div style="page-break-after:always"></div><!-- <div data-AmznPageBreak="always"> -->
<p title="data-AmznPageBreak='always'">Text</p></body></html>''')
        self.assertEqual(result["native_breaks"], 1)
        self.assertEqual(result["css_only_breaks"], 1)

    def test_raw_inspection_excludes_other_flows(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "mobi8").mkdir()
            main = b'<body><div data-AmznPageBreak="always"></div></body>'
            other = b'<div data-AmznPageBreak="always"></div>'
            (root / "mobi8/book.rawml").write_bytes(main + other)
            fdst = b"FDST" + struct.pack(">6I", 12, 2, 0, len(main), len(main), len(main + other))
            header = bytearray(78)
            struct.pack_into(">H", header, 76, 2)
            table = struct.pack(">I4xI4x", 94, 98)
            book = root / "book.azw3"
            book.write_bytes(header + table + b"TEST" + fdst)
            result = inspect_raw_breaks(book, root)
            self.assertEqual(result["main_flow_bytes"], len(main))
            self.assertEqual(result["native_breaks"], 1)
            book.write_bytes(header + table + b"TEST" + fdst[:19])
            with self.assertRaises(RuntimeError):
                inspect_raw_breaks(book, root)

    def test_report_audit_rejects_css_only_materialized_break(self):
        with tempfile.TemporaryDirectory() as directory:
            report = Path(directory) / "report.zip"
            for attribute, expected_errors in [('style="page-break-after:always"', 1), ('data-AmznPageBreak="always"', 0)]:
                with zipfile.ZipFile(report, "w") as archive:
                    archive.writestr("probe/book.azw3-documents.json", json.dumps([
                        {"fid": 0, "filename": "chapter.xhtml", "changes": [{"action": "materialized page-break-before:always"}]}]))
                    archive.writestr("probe/azw3-compiled/0000.xhtml", '<html><body><div ' + attribute + '></div></body></html>')
                self.assertEqual(len(audit_pagination_report(report)["errors"]), expected_errors)

    def test_self_closing_only_counts_actual_html_tokens(self):
        data = b'''<html xmlns="http://www.w3.org/1999/xhtml"><body>
<div id="closed"></div><div id="empty" title="a > b"/><br/>
<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>
</body></html>'''
        self.assertEqual(self_closing_nonvoid(data), [{"tag": "div", "id": "empty", "class": ""}])

    def test_inspection_preserves_missing_link_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            text = root / "mobi8/OEBPS/Text"
            text.mkdir(parents=True)
            (text / "one.xhtml").write_text('''<html><body><p id="start">Text</p>
<a href="two%20words.xhtml#target">Good</a><a href="missing.xhtml">Missing</a>
</body></html>''')
            (text / "two words.xhtml").write_text('<html><body><p id="target">Target</p></body></html>')
            result = inspect(root)
            self.assertEqual(len(result["documents"]), 2)
            self.assertEqual(result["self_closing_nonvoid"], 0)
            self.assertEqual(result["broken_links"], [{"file": "Text/one.xhtml", "href": "missing.xhtml"}])

    def test_json_configuration_requires_mapping(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({"document": {"azw3": {"compression": "standard"}}}))
            self.assertEqual(load_config(path)["document"]["azw3"]["compression"], "standard")
            path.write_text("[]")
            with self.assertRaises(RuntimeError):
                load_config(path)

    def test_command_log_keeps_bytes_and_exit_status(self):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / "command.log"
            status = run([sys.executable, "-c", "import sys; sys.stdout.buffer.write(b'\\xff'); sys.exit(1)"],
                         directory, log, allowed=(0, 1))
            self.assertEqual(status, 1)
            self.assertEqual(log.read_bytes(), b"\xff")

    def test_reference_uses_pre_lowering_snapshots_and_resource_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = root / "report.zip"
            source = b'<html><body><div class="body-title">Title</div><div class="emptyline"/></body></html>'
            asset = b"source image bytes"
            prefix = "fbc-test/"
            with zipfile.ZipFile(report, "w") as archive:
                archive.writestr(prefix + "book.azw3-documents.json", json.dumps([{"fid": 0, "filename": "Text/chapter.xhtml"}]))
                archive.writestr(prefix + "azw3-input/0000.xhtml", source)
                archive.writestr(prefix + "book.source.css", ".body-title {page-break-before:always}")
                archive.writestr(prefix + "book.azw3-resources.json", json.dumps([
                    {"filename": "images/a.png", "mime": "image/png", "input_data": "azw3-resources/image-0001.bin",
                     "sha256": hashlib.sha256(asset).hexdigest()}]))
                archive.writestr(prefix + "azw3-resources/image-0001.bin", asset)
                archive.writestr(prefix + "book.azw3-navigation.json", json.dumps(
                    {"title": "Test", "identifier": "test-id", "language": "en", "toc": None, "landmarks": None}))
            destination = root / "publication"
            opf = export_reference_publication(report, destination)
            self.assertEqual((destination / "Text/chapter.xhtml").read_bytes(), source)
            self.assertEqual((destination / "images/a.png").read_bytes(), asset)
            self.assertNotIn(b"page-break-after:always", (destination / "Text/chapter.xhtml").read_bytes())
            package = ET.parse(opf)
            hrefs = {e.get("href") for e in package.findall(".//{http://www.idpf.org/2007/opf}item")}
            self.assertEqual(hrefs, {"Text/chapter.xhtml", "images/a.png", "stylesheet.css", "reference.ncx"})


if __name__ == "__main__":
    unittest.main()
