#!/usr/bin/env python3
"""Build legacy, native and same-content KindleGen controls for manual KF8 parity checks.

No renderer is emulated: inspection/decoding results are not visual-parity assertions.
Optional YAML configs require PyYAML; defaults and JSON configs use only the stdlib.
"""

import argparse
from collections import Counter
import difflib
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
from urllib.parse import quote, unquote, urlsplit
import xml.etree.ElementTree as ET
from xml.parsers import expat
import zipfile


ROOT = Path(__file__).resolve().parent.parent
VOID = set("area base br col embed hr img input link meta param source track wbr".split())
XHTML = "http://www.w3.org/1999/xhtml"


def run(args, cwd, log, allowed=(0,)):
    result = subprocess.run([str(a) for a in args], cwd=cwd, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, timeout=180)
    log.write_bytes(result.stdout)
    if result.returncode not in allowed:
        raise RuntimeError(f"command failed ({result.returncode}); inspect {log}")
    return result.returncode


def load_config(path):
    text = path.read_text()
    try:
        result = json.loads(text)
    except json.JSONDecodeError:
        try:
            import yaml
        except ImportError as error:
            raise RuntimeError("YAML configs require PyYAML (or provide JSON instead)") from error
        result = yaml.safe_load(text)
    if result is None:
        return {}
    if not isinstance(result, dict):
        raise RuntimeError(f"configuration must contain a mapping: {path}")
    return result


def self_closing_nonvoid(data):
    result = []
    parser = expat.ParserCreate(namespace_separator="}")

    def start(name, attributes):
        namespace, _, tag = name.rpartition("}")
        if namespace not in ("", XHTML) or tag in VOID:
            return
        token = re.match(rb'''<(?:[^>"']|"[^"]*"|'[^']*')*>''', data[parser.CurrentByteIndex:])
        if token and token[0].endswith(b"/>"):
            result.append({"tag": tag, "id": attributes.get("id", ""), "class": attributes.get("class", "")})

    parser.StartElementHandler = start
    parser.Parse(data, True)
    return result


def inspect(directory):
    base = directory / "mobi8" / "OEBPS"
    documents, ids = [], {}
    for path in sorted(base.rglob("*.xhtml")):
        data = path.read_bytes()
        tree = ET.fromstring(data)
        counts, links = Counter(), []
        for element in tree.iter():
            tag = element.tag.rsplit("}", 1)[-1]
            counts[tag] += 1
            if tag == "a" and element.get("href"):
                links.append(element.get("href"))
        ids[path.resolve()] = {e.get("id") or e.get("name") for e in tree.iter()}
        text = " ".join(" ".join(tree.itertext()).split())
        documents.append({"file": str(path.relative_to(base)), "tags": dict(sorted(counts.items())),
                          "text_sha256": hashlib.sha256(text.encode()).hexdigest(), "text_characters": len(text),
                          "self_closing_nonvoid": self_closing_nonvoid(data), "links": links,
                          "explicit_breaks": data.count(b'page-break-after:always')})
    broken = []
    for document in documents:
        source = base / document["file"]
        for href in document["links"]:
            url = urlsplit(href)
            if url.scheme or url.netloc:
                continue
            target = (source.parent / unquote(url.path)).resolve() if url.path else source.resolve()
            if not target.is_file() or (url.fragment and target in ids and unquote(url.fragment) not in ids[target]):
                broken.append({"file": document["file"], "href": href})
    resources = [{"file": str(p.relative_to(base)), "bytes": p.stat().st_size,
                  "sha256": hashlib.sha256(p.read_bytes()).hexdigest()}
                 for p in sorted(base.rglob("*")) if p.is_file() and p.suffix not in (".xhtml", ".opf", ".ncx")]
    return {"documents": documents, "resources": resources, "broken_links": broken,
            "self_closing_nonvoid": sum(len(d["self_closing_nonvoid"]) for d in documents),
            "explicit_breaks": sum(d["explicit_breaks"] for d in documents)}


def inspect_break_markup(data):
    """Inspect reader input, not KindleUnpack's native-attribute-to-CSS rewrite."""
    markers = []

    class BreakParser(HTMLParser):
        def handle_starttag(self, tag, attributes):
            attributes = dict(attributes)
            native = (attributes.get("data-amznpagebreak") or "").lower() == "always"
            style = re.sub(r"\s+", "", attributes.get("style") or "").lower().rstrip(";")
            css_only = style == "page-break-after:always" and not native
            if native or css_only:
                line, column = self.getpos()
                markers.append({"tag": tag, "native": native, "css_only": css_only,
                                "line": line, "column": column, "markup": self.get_starttag_text()})

    parser = BreakParser()
    parser.feed(data.decode("utf-8"))
    parser.close()
    return {"native_breaks": sum(m["native"] for m in markers),
            "css_only_breaks": sum(m["css_only"] for m in markers), "markers": markers}


def inspect_native_anchors(data):
    """Keep reader-facing aids visible; KindleUnpack removes them from XHTML."""
    anchors, missing, table_notes = [], [], []

    class AnchorParser(HTMLParser):
        table_depth = 0

        def handle_starttag(self, tag, attributes):
            attributes = dict(attributes)
            if tag == "table":
                self.table_depth += 1
            aid = attributes.get("aid")
            if aid is not None:
                anchors.append({"tag": tag, "id": attributes.get("id", ""), "aid": aid})
            semantic = attributes.get("epub:type", "").split()
            if (tag == "a" and "noteref" in semantic) or (tag == "aside" and "footnote" in semantic):
                if self.table_depth:
                    table_notes.append({"tag": tag, "id": attributes.get("id", "")})
                elif not aid:
                    missing.append({"tag": tag, "id": attributes.get("id", "")})

        def handle_endtag(self, tag):
            if tag == "table":
                self.table_depth -= 1

    parser = AnchorParser()
    parser.feed(data.decode("utf-8"))
    parser.close()
    counts = Counter(a["aid"] for a in anchors)
    return {"count": len(anchors), "by_tag": dict(sorted(Counter(a["tag"] for a in anchors).items())),
            "duplicates": sorted(aid for aid, count in counts.items() if count > 1),
            "unanchored_notes": missing, "table_notes": table_notes, "anchors": anchors}


def palm_records(book):
    data = book.read_bytes()
    if len(data) < 78:
        raise RuntimeError("truncated PalmDB header: " + str(book))
    count = struct.unpack_from(">H", data, 76)[0]
    if not count or 78 + count * 8 > len(data):
        raise RuntimeError("invalid PalmDB record table: " + str(book))
    offsets = [struct.unpack_from(">I", data, 78 + i * 8)[0] for i in range(count)] + [len(data)]
    if offsets[0] < 78 + count * 8 or any(a >= b for a, b in zip(offsets, offsets[1:])):
        raise RuntimeError("invalid PalmDB record offsets: " + str(book))
    return [data[start:end] for start, end in zip(offsets, offsets[1:])]


def inspect_resc_spine(record):
    if len(record) < 16 or record[:4] != b"RESC":
        raise RuntimeError("invalid RESC header")
    header_size = struct.unpack_from(">I", record, 12)[0]
    header = record[16:16 + header_size]
    match = re.search(rb"(?:^|&)size=([0-9A-V]+)(?:&|$)", header)
    if not match:
        raise RuntimeError("missing RESC XML size")
    size = int(match[1], 32)
    start = 16 + header_size
    if start + size > len(record):
        raise RuntimeError("truncated RESC XML")
    # KindleGen stores adjacent metadata/spine XML fragments; native stores a
    # package root. Wrap either without changing namespace declarations.
    xml = re.sub(rb"<\?xml[^?]*\?>", b"", record[start:start + size])
    root = ET.fromstring(b"<root>" + xml + b"</root>")
    result = []
    for element in root.iter():
        if element.tag.rsplit("}", 1)[-1] == "spine":
            for item in element:
                if item.tag.rsplit("}", 1)[-1] == "itemref":
                    result.append({"idref": item.get("idref"), "skelid": item.get("skelid"), "linear": item.get("linear", "yes")})
    return result


def inspect_kindle_metadata(book):
    records = palm_records(book)
    with book.open("rb") as stream:
        database_name = stream.read(32).split(b"\0", 1)[0]
    header = records[0]
    if len(header) < 132 or header[16:20] != b"MOBI":
        raise RuntimeError("missing MOBI header: " + str(book))
    exth = 16 + struct.unpack_from(">I", header, 20)[0]
    if exth + 12 > len(header) or header[exth:exth + 4] != b"EXTH":
        raise RuntimeError("missing EXTH header: " + str(book))
    length, count = struct.unpack_from(">II", header, exth + 4)
    if length < 12 or exth + length > len(header):
        raise RuntimeError("invalid EXTH length: " + str(book))
    fields, at = {}, exth + 12
    for _ in range(count):
        if at + 8 > exth + length:
            raise RuntimeError("truncated EXTH record: " + str(book))
        key, size = struct.unpack_from(">II", header, at)
        if size < 8 or at + size > exth + length:
            raise RuntimeError("invalid EXTH record size: " + str(book))
        fields.setdefault(key, []).append(header[at + 8:at + size])
        at += size
    first_resource = struct.unpack_from(">I", header, 108)[0]
    start_reading = []
    for value in fields.get(116, []):
        if len(value) != 4:
            raise RuntimeError("invalid start-reading offset: " + str(book))
        start_reading.append(struct.unpack(">I", value)[0])

    def resource(key):
        if key not in fields:
            return None
        if len(fields[key]) != 1 or len(fields[key][0]) != 4:
            raise RuntimeError("invalid image resource offset: " + str(book))
        offset = struct.unpack(">I", fields[key][0])[0]
        record = first_resource + offset
        if record >= len(records):
            raise RuntimeError("image resource offset outside PalmDB: " + str(book))
        payload = records[record]
        return {"resource_offset": offset, "record": record, "bytes": len(payload),
                "sha256": hashlib.sha256(payload).hexdigest(), "prefix_hex": payload[:12].hex()}

    return {"content_type": [v.decode("utf-8") for v in fields.get(501, [])],
            "database_name": database_name.decode("cp1252"), "database_name_hex": database_name.hex(),
            "start_reading": start_reading,
            "guide_index": struct.unpack_from(">I", header, 0x104)[0] if len(header) >= 0x108 else None,
            "asin": [v.decode("utf-8") for v in fields.get(113, [])],
            "content_key": [v.decode("utf-8") for v in fields.get(504, [])],
            "cover": resource(201), "thumbnail": resource(202),
            "spine": [item for record in records if record[:4] == b"RESC" for item in inspect_resc_spine(record)]}


def inspect_raw_breaks(book, directory):
    """Bound the independently decompressed raw stream by its standalone PDB FDST."""
    lengths = []
    for record in palm_records(book):
        if record[:4] != b"FDST":
            continue
        if len(record) < 20:
            raise RuntimeError("truncated FDST: " + str(book))
        header, flows, main_start, main_end = struct.unpack_from(">4I", record, 4)
        if header != 12 or not flows or len(record) != header + flows * 8 or main_start != 0 or main_end == 0:
            raise RuntimeError("invalid FDST: " + str(book))
        lengths.append(main_end)
    if len(lengths) != 1:
        raise RuntimeError("expected one FDST in standalone KF8: " + str(book))
    streams = list((directory / "mobi8").glob("*.rawml"))
    if len(streams) != 1:
        raise RuntimeError("expected one independently decompressed KF8 raw stream: " + str(directory))
    raw = streams[0].read_bytes()
    if lengths[0] > len(raw):
        raise RuntimeError("FDST main flow exceeds decompressed raw stream: " + str(book))
    main = raw[:lengths[0]]
    return {"main_flow_bytes": lengths[0], **inspect_break_markup(main), "native_anchors": inspect_native_anchors(main)}


def audit_pagination_report(report_path):
    documents, errors = [], []
    with zipfile.ZipFile(report_path) as report:
        name = next(n for n in report.namelist() if n.endswith("/book.azw3-documents.json"))
        prefix = name.rsplit("/", 1)[0] + "/"
        for document in json.loads(report.read(name)):
            changes = [c for c in document["changes"] or []
                       if c["action"].startswith("materialized ") or c["action"] == "lowered explicit CSS page break"]
            markup = report.read(prefix + f"azw3-compiled/{document['fid']:04d}.xhtml")
            breaks = inspect_break_markup(markup)
            if breaks["native_breaks"] < len(changes):
                errors.append(f"{document['filename']}: {len(changes)} traced breaks but only "
                              f"{breaks['native_breaks']} native attributes")
            documents.append({"filename": document["filename"], "traced_breaks": len(changes), **breaks})
    return {"documents": documents, "errors": errors,
            "native_breaks": sum(d["native_breaks"] for d in documents)}


def unpack(book, output, log):
    run([ROOT / "testdata/run_kindleunpack.sh", "-r", "-d", "-s", book, output], ROOT, log)
    diagnostics = [line for line in log.read_text(errors="replace").splitlines()
                   if any(word in line.lower() for word in
                          ("warning", "error", "traceback", "retargeting", "incorrect insert position", "fixed corrupt fragment"))]
    if diagnostics:
        raise RuntimeError(f"KindleUnpack diagnostics must be inspected: {log}\n" + "\n".join(diagnostics))


def export_reference_publication(report_path, destination):
    """Use pre-lowering compiler inputs, not an already-compiled KF8 round trip.

    Refeeding explicit break markers through KindleGen is not idempotent and can
    multiply them. The debug source snapshots isolate exactly one compiler pass.
    """
    destination.mkdir()

    def write(filename, data):
        path = (destination / filename).resolve()
        if not path.is_relative_to(destination.resolve()):
            raise RuntimeError("publication path escapes reference directory: " + filename)
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists() and path.read_bytes() != data:
            raise RuntimeError("conflicting reference publication filename: " + filename)
        path.write_bytes(data)

    with zipfile.ZipFile(report_path) as report:
        names = [n for n in report.namelist() if n.endswith("/book.azw3-documents.json")]
        if len(names) != 1:
            raise RuntimeError("report must contain one AZW3 publication inventory")
        prefix = names[0].rsplit("/", 1)[0] + "/"
        documents = json.loads(report.read(names[0]))
        resources = json.loads(report.read(prefix + "book.azw3-resources.json")) or []
        navigation = json.loads(report.read(prefix + "book.azw3-navigation.json"))
        for document in documents:
            write(document["filename"], report.read(prefix + f"azw3-input/{document['fid']:04d}.xhtml"))
        write("stylesheet.css", report.read(prefix + "book.source.css"))
        for resource in resources:
            data = report.read(prefix + resource["input_data"])
            if hashlib.sha256(data).hexdigest() != resource["sha256"]:
                raise RuntimeError("source resource checksum mismatch: " + resource["filename"])
            write(resource["filename"], data)

    dc = "http://purl.org/dc/elements/1.1/"
    ET.register_namespace("dc", dc)
    package = ET.Element("package", {"xmlns": "http://www.idpf.org/2007/opf", "version": "2.0", "unique-identifier": "BookId"})
    metadata = ET.SubElement(package, "metadata")
    identifier = navigation.get("identifier", "parity-reference")
    ET.SubElement(metadata, "{" + dc + "}identifier", {"id": "BookId"}).text = identifier
    ET.SubElement(metadata, "{" + dc + "}title").text = navigation["title"]
    ET.SubElement(metadata, "{" + dc + "}language").text = navigation.get("language", "en")
    manifest = ET.SubElement(package, "manifest")
    for i, document in enumerate(documents):
        ET.SubElement(manifest, "item", {"id": f"doc{i}", "href": quote(document["filename"]), "media-type": "application/xhtml+xml"})
    ET.SubElement(manifest, "item", {"id": "css", "href": "stylesheet.css", "media-type": "text/css"})
    ET.SubElement(manifest, "item", {"id": "ncx", "href": "reference.ncx", "media-type": "application/x-dtbncx+xml"})
    for i, resource in enumerate(resources):
        ET.SubElement(manifest, "item", {"id": f"asset{i}", "href": quote(resource["filename"]), "media-type": resource["mime"]})
        if resource["filename"] == navigation.get("cover_filename"):
            ET.SubElement(metadata, "meta", {"name": "cover", "content": f"asset{i}"})
    cover_document = next((p["Href"].split("#")[0] for p in (navigation.get("landmarks") or []) if p["Kind"] == "cover"), None)
    spine = ET.SubElement(package, "spine", {"toc": "ncx"})
    for i, document in enumerate(documents):
        attributes = {"idref": f"doc{i}"}
        if document["filename"] == cover_document:
            attributes["linear"] = "no"
        ET.SubElement(spine, "itemref", attributes)
    guide = ET.SubElement(package, "guide")
    for landmark in navigation.get("landmarks") or []:
        ET.SubElement(guide, "reference", {"type": landmark["Kind"], "title": landmark["Title"], "href": landmark["Href"]})
    write("reference.opf", ET.tostring(package, encoding="utf-8", xml_declaration=True))

    ncx = ET.Element("ncx", {"xmlns": "http://www.daisy.org/z3986/2005/ncx/", "version": "2005-1"})
    head = ET.SubElement(ncx, "head")
    ET.SubElement(head, "meta", {"name": "dtb:uid", "content": identifier})
    ET.SubElement(ET.SubElement(ncx, "docTitle"), "text").text = navigation["title"]
    navmap = ET.SubElement(ncx, "navMap")
    stack = [(float("-inf"), navmap)]
    for i, item in enumerate(navigation.get("toc") or []):
        while len(stack) > 1 and item["Level"] <= stack[-1][0]:
            stack.pop()
        point = ET.SubElement(stack[-1][1], "navPoint", {"id": f"nav{i}", "playOrder": str(i + 1)})
        ET.SubElement(ET.SubElement(point, "navLabel"), "text").text = item["Title"]
        ET.SubElement(point, "content", {"src": item["Href"]})
        stack.append((item["Level"], point))
    write("reference.ncx", ET.tostring(ncx, encoding="utf-8", xml_declaration=True))
    return destination / "reference.opf"


def main():
    tools = ROOT.parents[2] / "book-tests"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("book", type=Path)
    parser.add_argument("output", type=Path, help="new/empty run directory (never replaces an earlier run)")
    parser.add_argument("--native", type=Path, default=ROOT / "build/fbc")
    parser.add_argument("--legacy", type=Path, default=tools / "fb2converter/fb2c")
    parser.add_argument("--kindlegen", type=Path, default=tools / "fb2converter/kindlegen")
    parser.add_argument("--native-config", type=Path)
    parser.add_argument("--legacy-config", type=Path)
    parser.add_argument("--footnotes", choices=("default", "float", "floatRenumbered"), default="default")
    args = parser.parse_args()
    for name in ("book", "output", "native", "legacy", "kindlegen"):
        setattr(args, name, getattr(args, name).resolve())
    if not args.book.is_file():
        parser.error("input book does not exist")
    if args.output.exists() and any(args.output.iterdir()):
        parser.error("output directory is not empty; use a new run directory")
    args.output.mkdir(parents=True, exist_ok=True)
    work = args.output
    manifest = {"input": str(args.book), "input_sha256": hashlib.sha256(args.book.read_bytes()).hexdigest(),
                "footnotes": args.footnotes,
                "tools": {name: {"path": str(getattr(args, name)),
                                 "sha256": hashlib.sha256(getattr(args, name).read_bytes()).hexdigest()}
                          for name in ("native", "legacy", "kindlegen")}}
    (work / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    for name in ("legacy", "native", "view"):
        (work / name).mkdir()

    legacy_config = work / "legacy/config.json"
    run([args.legacy, "dumpconfig", legacy_config], work / "legacy", work / "legacy/config.log")
    cfg = load_config(args.legacy_config) if args.legacy_config else load_config(legacy_config)
    document = cfg.setdefault("document", {})
    document.setdefault("kindlegen", {})["path"] = str(args.kindlegen)
    document["kindlegen"]["verbose"] = True
    document.setdefault("notes", {})["mode"] = "default" if args.footnotes == "default" else "float-new"
    document["notes"]["renumber"] = args.footnotes == "floatRenumbered"
    cfg.setdefault("logger", {})["file"] = {"level": "debug", "mode": "overwrite", "destination": str(work / "legacy/convert.log")}
    legacy_config.write_text(json.dumps(cfg, indent=2))
    run([args.legacy, "-d", "-c", legacy_config, "convert", "--to", "azw3", "--nodirs", args.book, work / "legacy"],
        work / "legacy", work / "legacy/generation.log")
    legacy_books = list((work / "legacy").glob("*.azw3"))
    if len(legacy_books) != 1:
        raise RuntimeError("legacy converter did not produce exactly one AZW3")

    cfg = load_config(args.native_config) if args.native_config else {}
    cfg.setdefault("document", {}).setdefault("footnotes", {})["mode"] = args.footnotes
    cfg.setdefault("logging", {})["file"] = {
        "level": "debug", "mode": "overwrite", "destination_template": str(work / "native/convert.log"),
        "panic_destination_template": str(work / "native/panic.log")}
    cfg["reporting"] = {"destination_template": str(work / "native/report.zip")}
    native_config = work / "native/config.json"
    native_config.write_text(json.dumps(cfg, indent=2))
    native_book = work / "native/book.azw3"
    run([args.native, "-d", "-c", native_config, "convert", "--to", "azw3", "--output-file", native_book, args.book],
        ROOT, work / "native/generation.log")
    inspections = {}
    for name, book in (("legacy", legacy_books[0]), ("native", native_book)):
        unpack(book, work / (name + "-unpacked"), work / (name + "-unpack.log"))
        inspections[name] = inspect(work / (name + "-unpacked"))
        shutil.copy2(book, work / "view" / (name + ".azw3"))

    source_opf = export_reference_publication(work / "native/report.zip", work / "reference-source")
    status = run([args.kindlegen, source_opf, "-c1", "-o", "reference.mobi"], work,
                 work / "same-content-kindlegen.log", allowed=(0, 1))
    unpack(source_opf.parent / "reference.mobi", work / "reference-unpacked", work / "reference-unpack.log")
    inspections["reference"] = inspect(work / "reference-unpacked")
    books = list((work / "reference-unpacked").glob("*.azw3"))
    if len(books) != 1:
        raise RuntimeError("KindleUnpack did not split out one standalone reference AZW3")
    shutil.copy2(books[0], work / "view/reference.azw3")

    for name in ("legacy", "native", "reference"):
        inspections[name]["raw_pagination"] = inspect_raw_breaks(work / "view" / (name + ".azw3"), work / (name + "-unpacked"))
        inspections[name]["kindle_metadata"] = inspect_kindle_metadata(work / "view" / (name + ".azw3"))
    pagination_audit = audit_pagination_report(work / "native/report.zip")
    if pagination_audit["native_breaks"] != inspections["native"]["raw_pagination"]["native_breaks"]:
        pagination_audit["errors"].append("compiled snapshots and independently decoded raw stream have different native-break counts")
    (work / "native-pagination-audit.json").write_text(json.dumps(pagination_audit, ensure_ascii=False, indent=2) + "\n")
    for name, result in inspections.items():
        (work / (name + "-inspection.json")).write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    source_css = (work / "native-unpacked/mobi8/OEBPS/Styles/style0001.css").read_text()
    compiled_css = (work / "reference-unpacked/mobi8/OEBPS/Styles/style0001.css").read_text()
    (work / "same-content-css.diff").write_text("".join(difflib.unified_diff(
        source_css.splitlines(True), compiled_css.splitlines(True), fromfile="native", tofile="kindlegen")))
    with zipfile.ZipFile(work / "native/report.zip") as report:
        names = report.namelist()
        for required in ("book.azw3-documents.json", "book.azw3-anchors.json", "book.azw3-style-trace.txt",
                         "book.azw3-resources.json", "book.azw3-flows.json", "book.azw3-records.json", "book.azw3-navigation.json"):
            if not any(name.endswith("/" + required) for name in names):
                raise RuntimeError("native debug report lacks " + required)
    rows = ["# AZW3 comparison", "", f"Input: `{args.book}`", f"Footnotes: `{args.footnotes}`", "",
            "| Arm | Documents | Self-closing non-void | Decoded CSS breaks | Raw native breaks | Raw CSS-only breaks | Element aids | Unresolved links |",
            "|---|---:|---:|---:|---:|---:|---:|---:|"]
    for name, result in inspections.items():
        rows.append(f"| {name} | {len(result['documents'])} | {result['self_closing_nonvoid']} | "
                    f"{result['explicit_breaks']} | {result['raw_pagination']['native_breaks']} | "
                    f"{result['raw_pagination']['css_only_breaks']} | "
                    f"{result['raw_pagination']['native_anchors']['count']} | {len(result['broken_links'])} |")
    rows += ["", f"Same-content KindleGen exit status: {status} (1 means warnings; inspect the complete log).", "",
             "Open view/legacy.azw3, view/native.azw3 and view/reference.azw3 in the same Kindle Previewer profile.",
             "Compare headings, epigraphs, empty lines, poems, tables, images, links and floating notes.",
             "Different converter defaults/classes are not automatically compiler defects.",
             "The reference is built from native debug pre-lowering XHTML/CSS/resource snapshots, not from already-compiled markup.",
             "Raw native breaks count data-AmznPageBreak attributes in the FDST-bounded main flow; decoded CSS divs are not equivalent.",
             "Element aids are reader-facing native anchor identities, also stripped during reconstruction; inspect raw_pagination.native_anchors.",
             "Inspect same-content-css.diff for KindleGen CSS preprocessing and the JSON inventories for markup/resources.",
             "Decoding and this report do not establish visual parity."]
    (work / "SUMMARY.md").write_text("\n".join(rows) + "\n")
    if pagination_audit["errors"]:
        raise RuntimeError("native pagination audit failed; inspect native-pagination-audit.json")
    anchors = inspections["native"]["raw_pagination"]["native_anchors"]
    if anchors["duplicates"] or anchors["unanchored_notes"]:
        raise RuntimeError("native element anchor audit failed; inspect native-inspection.json")
    for name in ("native", "reference"):
        if inspections[name]["broken_links"]:
            raise RuntimeError(f"{name} has unresolved local links; inspect {name}-inspection.json")
    if inspections["legacy"]["broken_links"]:
        print("Legacy baseline has unresolved local links; details retained in legacy-inspection.json.", file=sys.stderr)
    if inspections["native"]["self_closing_nonvoid"]:
        raise RuntimeError("native output still contains self-closing non-void HTML elements")
    print(work / "SUMMARY.md")


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, subprocess.TimeoutExpired, ValueError) as error:
        print(f"AZW3 comparison failed: {error}", file=sys.stderr)
        sys.exit(1)
