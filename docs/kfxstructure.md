# KFX Structure (reverse engineered from KFXInput/KFXOutput calibre plugins and KPV files)

Much of the knowledge of the KPF internals comes from Calibre's KFX conversion Input Plugin v2.27.1 created by John Howell <jhowell@acm.org> and copyrighted under GPL v3. Visit https://www.mobileread.com/forums for more details.

This document describes the parts of the KFX on-disk format. It focuses on:
- The outer KFX container format (`CONT` + `ENTY`)
- The embedded Amazon Ion Binary encoding used for most payloads
- The fragment model used by the “YJ” data model (fragment type + fragment id)
- High-level schemas for the most important fragment types used by this converter

Where possible, each fact includes a pointer to the relevant parsing/validation code (file + symbol/function).

---

## 1. Files, packaging, and discovery

The loader accepts multiple “package shapes”:

- Single container files: `.kfx`, `.azw8`, `.ion`, `.kpf`
- KFX-ZIP style: a `.zip` / `.kfx-zip` that contains `book.ion` (Ion text) and resources
- A directory tree containing multiple `.kfx` / `.res` / `.yj` / etc files (e.g. “.sdr” bundles)

Derived from: `kfxlib/yj_book.py:YJ_Book.locate_book_datafiles`, `kfxlib/yj_book.py:YJ_Book.get_container`.

### 1.1 Container type selection

A datafile is treated as:

- **Ion text container** when the file extension is `.ion` and the bytes do not start with the Ion binary signature.
- **Zip-unpack container** when the file is a ZIP and contains `book.ion` (Ion text)
- **KPF** when the file is a SQLite `KDF` or ZIP containing `.kdf`
- **KFX `CONT`** when the bytes start with `CONT`
- **DRM-wrapped KFX** when the bytes start with the DRMION signature `\xeaDRMION\xee`

Derived from: `kfxlib/yj_book.py:YJ_Book.get_container`, `kfxlib/yj_container.py:DRMION_SIGNATURE`, `kfxlib/kfx_container.py:KfxContainer.SIGNATURE`.

---

## 2. Amazon Ion usage in KFX

KFX uses **Amazon Ion 1.0** values as the primary encoding for structured data.
The project implements two codecs:

- **Ion Binary** (`IonBinary`): used inside real `.kfx` containers and in some KPF blobs
- **Ion Text** (`IonText`): used inside “zip-unpack” `book.ion` containers and for symbol catalogs

Derived from: `kfxlib/ion_binary.py:IonBinary`, `kfxlib/ion_text.py:IonText`.

### 2.1 Ion Binary stream framing

- Each Ion Binary stream begins with 4 bytes: `E0 01 00 EA` (version marker + major/minor + EA).
- A stream contains one or more values.
- Between values, the decoder tolerates an embedded Ion signature only when the next byte equals `0xE0`: it then reads 4 bytes and requires them to equal `E0 01 00 EA`, otherwise parsing fails.

Derived from: `kfxlib/ion_binary.py:IonBinary.SIGNATURE`, `IonBinary.deserialize_multiple_values_`.

### 2.2 Ion Binary value encoding (as implemented here)

Each value starts with a one-byte **descriptor**:
- high nibble: **type signature** (0..15)
- low nibble: **flag** (0..15)

If the flag is:
- `< 14`: it is the length in bytes of the value body
- `14`: the length is an Ion VLUInt immediately following the descriptor
- `15`: “null” for the given signature (except signature 0 which is null itself)

The implementation supports the standard Ion signatures 0..15: null, bool, int, float, decimal, timestamp, symbol, string, clob, blob, list, sexp, struct, annotation, reserved.

Derived from: `kfxlib/ion_binary.py:IonBinary.deserialize_value`, `VALUE_DESERIALIZERS`.

### 2.3 Symbol IDs and symbol tables

Ion “symbols” are represented in this project as `IonSymbol` objects (a `str` subclass).
Most KFX semantics are expressed via symbol IDs like `$270` rather than literal strings.

The local symbol table (`LocalSymbolTable`) supports:
- Importing shared symbol tables (`$ion` and `YJ_symbols`)
- Local symbols (`symbols` list)
- A translation layer (optional external symbol catalog) to map placeholder `$NNN` names to readable names

Derived from: `kfxlib/ion_symbol_table.py:LocalSymbolTable`, `kfxlib/yj_symbol_catalog.py:YJ_SYMBOLS`, `kfxlib/yj_book.py:YJ_Book.load_symbol_catalog`.

#### 2.3.1 Symbol ID numbering schemes (KFX vs Standard Ion)

**CRITICAL**: KFX files use a non-standard symbol ID numbering scheme that differs from standard Ion implementations.

**KFX numbering (used by kfxlib and Kindle readers)**:
- IDs 1-851: YJ_symbols shared symbol table (`$10` to `$860`)
- IDs 852+: Local symbols (book-specific names like chapter IDs, style names)
- **Ion system symbols (1-9) are NOT counted** in the ID space

**Standard Ion numbering (used by Amazon Ion SDK, including Go's ion-go)**:
- IDs 1-9: Ion system symbols (`$ion_symbol_table`, `name`, `version`, etc.)
- IDs 10-860: YJ_symbols (after importing with `max_id: 851`)
- IDs 861+: Local symbols

This 9-ID offset affects:
1. **Entity directory**: `id_idnum` and `type_idnum` use KFX numbering (852+ for local symbols)
2. **Doc symbol table `max_id`**: Stored with Ion system symbol offset, must be adjusted when reading/writing
3. **Symbol values in payloads**: Written with KFX numbering, require manual resolution when reading

**Example**:
- A local symbol at index 0 (e.g., "chapter_1"):
  - KFX ID: 852 (LargestKnownSymbol + 1 = 851 + 1)
  - Standard Ion ID: 861 (after $ion system symbols + YJ_symbols)
- When reading a payload with symbol ID 852, if the doc symbol table shows ID 861, manual resolution using the local symbols list is required.

Derived from: `convert/kfx/container.go:GetLocalSymbolID`, `convert/kfx/ionutil.go:createCombinedSymbolTable`, `convert/kfx/ionutil.go:IonReader.SymbolValue`.

---

## 3. KFX “CONT” container format

This section documents the **single-file** KFX container format used by `.kfx` files whose bytes begin with ASCII `CONT`.
It is an **implementation-derived** specification of what this repository reads/writes.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`, `kfxlib/kfx_container.py:KfxContainer.serialize`.

### 3.1 File layout overview (as implemented here)

A `CONT` file is conceptually split into two regions:

1. **Header region**: `file[0 : header_len]`
2. **Entity payload region**: `file[header_len : end]` (concatenation of entity records)

Important: in this implementation, `header_len` is the **absolute file offset** of the first entity record.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize` (`payload_sha1 = sha1(data[header_len:])`, `entity_start = header_len + entity_offset`).

### 3.2 Fixed header (18 bytes) and field semantics

All fixed-header integer fields are **little-endian**.

Byte layout (offsets in hex, sizes in bytes):

- `0x00` (4): `signature` = `CONT`
- `0x04` (2): `version` (u16)
- `0x06` (4): `header_len` (u32)
- `0x0A` (4): `container_info_offset` (u32)
- `0x0E` (4): `container_info_length` (u32)

Sanity rules enforced by this implementation:

- If the overall file length is `< 18`, parsing fails.
- If `signature != CONT`, parsing fails (with a special-case error message if bytes at `file[64:68]` suggest a PDB/MOBI container).
- If `version` is not 1 or 2, an error is logged (parsing continues).
- If `header_len < 18`, parsing fails.

Derived from: `kfxlib/kfx_container.py:KfxContainer.MIN_LENGTH`, `ALLOWED_VERSIONS`, and the corresponding checks in `KfxContainer.deserialize`.

Notes:

- The code does not explicitly require `container_info_offset < header_len`, but it slices the `container_info` bytes from the file directly using that pair.
- The code uses `header_len` as the start of the entity region and as the end of the header metadata blob.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`.

### 3.3 `container_info` (Ion Binary single value: struct)

`container_info` is an **Ion Binary** stream containing exactly one top-level value (a struct) located at:

- `file[container_info_offset : container_info_offset + container_info_length]`

This struct contains:

- Basic container identity/configuration values
- Absolute offsets + lengths (within the file) for other header components

Fields accessed by this implementation:

- `$409` (`bcContId`): container identifier string
- `$410` (`bcComprType`): compression type; read default `0`, logs error if non-zero
- `$411` (`bcDRMScheme`): DRM scheme; read default `0`, logs error if non-zero
- `$412` (`bcChunkSize`): chunk size; read default `4096`, logs warning if not `4096`
- `$413`/`$414` (`bcIndexTabOffset`/`bcIndexTabLength`): absolute offset + length of the entity directory
- `$415`/`$416` (`bcDocSymbolOffset`/`bcDocSymbolLength`): absolute offset + length of the embedded doc symbol table (Ion value annotated `$ion_symbol_table`)
- `$594`/`$595` (`bcFCapabilitiesOffset`/`bcFCapabilitiesLength`): absolute offset + length of the embedded format capabilities value (Ion value annotated `$593`, only for `version > 1`)

Validation and consumption rules (exact behavior):

- `$409` is read with default `""` and then removed from the struct.
- `$410` and `$411` are read with default `0`; if not `0`, an error is logged.
- `$412` is read with default `4096`; if not `4096`, a warning is logged.
- `$415`/`$416` are read; if length is non-zero, the doc symbol blob is parsed and imported into the current symbol table.
- If `version > 1`, `$594`/`$595` are read; if length is non-zero, the format capabilities blob is parsed.
- `$413`/`$414` are read and then used to parse the entity directory.
- After popping known keys, if any keys remain in the struct, an error is logged (“extra data”).

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`.

### 3.3.1 `container_info` field glossary (as used by this repo)

This is a compact, implementation-faithful glossary for the `container_info` struct keys that this repo reads/writes.
The `b.jad` names are provided for convenience; the wire identifiers are the numeric `$NNN` ids.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`, `kfxlib/kfx_container.py:KfxContainer.serialize`, and `b.jad`.

| Symbol | b.jad Enum | b.jad String | Read behavior | Write behavior |
|---:|---|---|---|---|
| `$409` | `ContainerId` | `bcContId` | `container_id = pop($409, "")` | Written from `$270.$409` (container fragment) |
| `$410` | `CompressionType` | `bcComprType` | `pop($410, 0)`; logs error if non-zero | Always written as `0` |
| `$411` | `DRMScheme` | `bcDRMScheme` | `pop($411, 0)`; logs error if non-zero | Always written as `0` |
| `$412` | `ChunkSize` | `bcChunkSize` | `pop($412, 4096)`; logs warning if not 4096 | Always written as `4096` |
| `$413` | `IndexTabOffset` | `bcIndexTabOffset` | `pop($413, None)` | Written as the absolute offset where the entity directory starts (`len(container)` at that moment) |
| `$414` | `IndexTabLength` | `bcIndexTabLength` | `pop($414, 0)`; if non-zero, used to read entity directory | Written as byte length of the entity directory (`len(entity_table)`) |
| `$415` | `DocSymOffset` | `bcDocSymbolOffset` | `pop($415, None)` | Written as the absolute offset where the doc symbol blob starts |
| `$416` | `DocSymLength` | `bcDocSymbolLength` | `pop($416, 0)`; if non-zero, parses `$ion_symbol_table` value and imports symbols | Written as byte length of the doc symbol blob |
| `$594` | `FCapabilitiesOffset` | `bcFCapabilitiesOffset` | Only read when `version > 1`; `pop($594, None)` | Only written when `self.symtab.local_min_id > 595`; set to absolute offset where `$593` blob starts |
| `$595` | `FCapabilitiesLength` | `bcFCapabilitiesLength` | Only read when `version > 1`; `pop($595, 0)`; if non-zero parses annotated `$593` value | Only written when `self.symtab.local_min_id > 595`; set to byte length of the `$593` blob |

Notes:

- The reader treats any remaining keys in `container_info` after popping known ones as “extra data” (logged as an error).
- The writer always emits `CONT` version 2 in the fixed header, regardless of what was originally read.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`, `kfxlib/kfx_container.py:KfxContainer.VERSION`, `kfxlib/kfx_container.py:KfxContainer.serialize`.

### 3.4 Entity directory (`bcIndexTabOffset`/`bcIndexTabLength`)

If `$414` (`bcIndexTabLength`) is non-zero, the entity directory is a packed sequence of fixed-size entries, stored at:

- `file[index_table_offset : index_table_offset + index_table_length]`

Each entry has this binary layout (all little-endian):

- `id_idnum` (u32): Ion symbol-id number for the fragment id
- `type_idnum` (u32): Ion symbol-id number for the fragment type
- `entity_offset` (u64): **offset relative to `header_len`** where the entity record begins
- `entity_len` (u64): length of the entity record in bytes

There is no explicit entry-count; parsing continues until the directory slice is exhausted.
The implementation reads 24 bytes per entry (`<L`, `<L`, `<Q`, `<Q`). If `index_table_length` is not a multiple of 24, parsing will eventually fail when `struct.unpack_from(...)` runs past the end of the directory slice.

Entity record location:

- `entity_start = header_len + entity_offset`
- `entity_bytes = file[entity_start : entity_start + entity_len]`

Bounds checking:

- If `entity_start + entity_len > len(file)`, parsing fails.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize` (unpacks `<L`, `<L`, `<Q`, `<Q` and computes `entity_start = header_len + entity_offset`).

### 3.5 Entity payload region (`file[header_len:]`)

The entity payload region is the concatenation of `ENTY` records referenced by the directory. This implementation also computes:

- `payload_sha1 = sha1(file[header_len:]).hex()`

and verifies it against the `kfxgen_payload_sha1` field in the header metadata blob.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`.

### 3.6 Embedded doc symbol table blob (`bcDocSymbolOffset`/`bcDocSymbolLength`)

If `$416` (`bcDocSymbolLength`) is non-zero:

- The blob is parsed as an **Ion Binary** *annotated value* with annotation `$ion_symbol_table`.
- After parsing, each import in `value["imports"]` that contains a `max_id` has that `max_id` adjusted **down** by `len(SYSTEM_SYMBOL_TABLE.symbols)` (typically 9).
- The resulting symbol table definition is then fed to `self.symtab.create(...)`.

This implies the on-disk doc symbol table uses `max_id` values that include the Ion system symbol table width, while the in-memory `LocalSymbolTable` here wants `max_id` relative to its own numbering.

**Relationship to symbol ID numbering (see §2.3.1)**:

The doc symbol table `max_id` adjustment is part of the KFX vs Standard Ion numbering difference:
- On disk: YJ_symbols import has `max_id: 860` (includes 9 Ion system symbols)
- In memory (kfxlib): YJ_symbols `max_id` is 851 (excludes Ion system symbols)
- When standard Ion libraries read the doc symbol table, local symbols start at ID 861
- But entity directory and payload symbol values use KFX numbering where local symbols start at ID 852

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize` (doc symbol import adjustment) and `kfxlib/yj_symbol_catalog.py:SYSTEM_SYMBOL_TABLE`.

Writer behavior:

- When serializing, the code expects there to be exactly one `$ion_symbol_table` fragment (it logs an error if not).
- Before writing the blob to the container, it deep-copies the value and adjusts any import `max_id` **up** by `len(SYSTEM_SYMBOL_TABLE.symbols)`.

Derived from: `kfxlib/kfx_container.py:KfxContainer.serialize`.

### 3.7 Embedded format capabilities blob (`bcFCapabilitiesOffset`/`bcFCapabilitiesLength`, KFX v2)

If `version > 1` and `$595` (`bcFCapabilitiesLength`) is non-zero:

- The blob is parsed as an **Ion Binary** *annotated value* with annotation `$593`.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`.

Writer behavior (important subtlety):

- The serializer always writes `version = 2` (`KfxContainer.VERSION`).
- It only writes `$594`/`$595` and includes the format capabilities blob if `self.symtab.local_min_id > 595`.
- If it does not include format capabilities, it omits those keys from `container_info` and does not append any format capabilities bytes.

Derived from: `kfxlib/kfx_container.py:KfxContainer.VERSION`, `KfxContainer.serialize`.

### 3.8 `kfxgen` header metadata blob (JSON-like, ends at `header_len`)

This implementation expects a metadata blob occupying exactly:

- `file[container_info_offset + container_info_length : header_len]`

Normalization rules applied before JSON parsing:

- All byte `0x1B` values are removed.
- The remaining bytes are decoded as ASCII with `errors="ignore"`.
- The decoded text is turned into valid JSON by rewriting:
   - `key :` and `key:` → `"key":`
   - `value:` → `"value":`

The resulting JSON must deserialize into a list of objects, each containing exactly:

- `key` (string)
- `value` (string)

Known keys accepted by this implementation:

- `appVersion` or `kfxgen_application_version` → recorded as generator application version
- `buildVersion` or `kfxgen_package_version` → recorded as generator package/build version
- `kfxgen_payload_sha1` → must match `sha1(file[header_len:]).hex()` (otherwise error logged)
- `kfxgen_acr` → must match `$409` container id (otherwise error logged)

Any other key is logged as “unknown”. Any extra fields within the per-entry object are logged as “extra data”.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize`.

Writer behavior:

- The writer constructs a JSON list of objects with keys:
   - `kfxgen_package_version`
   - `kfxgen_application_version`
   - `kfxgen_payload_sha1` (SHA1 of the entity payload region it is about to append)
   - `kfxgen_acr` (container id)
- It uses compact JSON serialization, then rewrites `"key":` → `key:` and `"value":` → `value:`.

Derived from: `kfxlib/kfx_container.py:KfxContainer.serialize`.

### 3.9 Concrete header byte ordering written by this serializer

The serializer writes the header in this exact order:

1. Fixed header with placeholder `header_len`, placeholder `container_info_offset`, placeholder `container_info_length`
2. Entity directory bytes (the packed table)
3. Doc symbol table bytes (byte length is permitted to be `0`)
4. Format capabilities bytes (optional)
5. `container_info` Ion struct bytes
6. `kfxgen` metadata blob bytes
7. `header_len` is then patched to equal the total bytes written so far
8. The entity payload region is appended (all serialized `ENTY` records)

Derived from: `kfxlib/kfx_container.py:KfxContainer.serialize`, `kfxlib/utilities.py:Serializer.repack`.

### 3.10 Container “type” inference (main/metadata/attachable)

After reading the entity directory, the implementation classifies the container by inspecting the set of `type_idnum` values present:

- If any entity has type idnum in `{259, 260, 538}` → `"KFX main"`
- Else if any entity has type idnum in `{258, 419, 490, 585}` **or** if doc symbol table length is non-zero → `"KFX metadata"`
- Else if any entity has type idnum in `{417}` → `"KFX attachable"`
- Else → error logged and the container is labeled `"KFX unknown"`

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize` and the `*_CONTAINER_FRAGMENT_IDNUMS` constants.

### 3.11 DRMION-wrapped containers (not `CONT`)

If the file begins with the `DRMION` signature (`\xEA DRMION \xEE`), this implementation treats it as DRM-protected and does not parse it as `CONT` directly.

Exact dispatch behavior:

- Only when the filename ends with `metadata.kfx`, it attempts to “expand” the DRMION wrapper using an external DeDRM component; if expansion produces bytes that begin with `CONT`, it re-dispatches parsing on the expanded bytes.
- Otherwise: if `ignore_drm=True` it returns no container; if `ignore_drm=False` it raises `KFXDRMError`.

Derived from: `kfxlib/yj_book.py:YJ_Book.get_container`, `kfxlib/yj_book.py:YJ_Book.expand_compressed_container`, `kfxlib/yj_container.py:DRMION_SIGNATURE`.

---

## 4. Entity record (“ENTY”) format

This section describes the `ENTY` record format and how it is mapped to fragments.

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.deserialize`, `kfxlib/kfx_container.py:KfxContainerEntity.serialize`, `kfxlib/kfx_container.py:KfxContainer.serialize`.

### 4.1 Byte layout

All integer fields in the fixed portion are **little-endian**.

Offsets/sizes (bytes):

- `0x00` (4): `signature` = ASCII `ENTY`
- `0x04` (2): `version` (u16)
- `0x06` (4): `header_len` (u32)
- `0x0A` (`header_len - 10`): `entity_info` bytes (Ion Binary: single value)
- `header_len`..end: `entity_data` bytes (payload)

Important: an `ENTY` record does **not** contain its own total length field; its record length comes from the entity directory entry (`entity_len`) in the surrounding `CONT` file.

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize` (directory provides entity length) and `kfxlib/kfx_container.py:KfxContainerEntity.deserialize` (parses header_len then consumes “rest of slice” as entity_data).

### 4.2 Sanity/validation rules enforced by this implementation

- If `signature != ENTY`, parsing fails.
- If `version` is not 1, an error is logged (parsing continues).
- If `header_len < 10`, parsing fails.
- The `entity_info` must be a struct with only known keys; any extra keys cause parsing to fail.

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.MIN_LENGTH`, `ALLOWED_VERSIONS`, and checks in `KfxContainerEntity.deserialize`.

### 4.3 `entity_info` (Ion Binary single value: struct)

The `entity_info` value is decoded from the bytes `file[0x0A : header_len]` of the entity record and is required (by this implementation) to contain:

- `$410` (`bcComprType`) (default 0)
- `$411` (`bcDRMScheme`) (default 0)

Exact behavior:

- `$410` is popped with default `0`; if not `0`, an error is logged.
- `$411` is popped with default `0`; if not `0`, an error is logged.
- If the struct has any keys remaining after popping, parsing fails.

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.deserialize`.

### 4.4 Payload decoding (`entity_data`)

The payload bytes are `entity_data = file[header_len : end_of_record]`.

The surrounding container’s entity directory supplies:

- `id_idnum` (u32): fragment id symbol-id
- `type_idnum` (u32): fragment type symbol-id

These are resolved via the active symbol table to:

- `fid = symtab.get_symbol(id_idnum)`
- `ftype = symtab.get_symbol(type_idnum)`

Decoding depends on `ftype`:

- If `ftype` is in `RAW_FRAGMENT_TYPES = {"$417", "$418"}` then the payload is stored as `IonBLOB(entity_data)` (raw bytes).
- Otherwise, the payload is decoded as **Ion Binary**: one top-level value.

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.deserialize`, `kfxlib/yj_container.py:RAW_FRAGMENT_TYPES`.

### 4.5 IonAnnotation payload special case

If the decoded payload is an `IonAnnotation`:

- If it has exactly one annotation and that annotation equals `ftype`, and the `fid` resolved from `id_idnum` equals `$348`, the code treats this as a “root fragment encoded with an annotated payload” and normalizes it by:
   - setting `fid = ftype`
   - replacing the value with the annotation’s inner value
- Otherwise, an error is logged (“Entity ... has IonAnnotation as value”).

Note: this normalization changes the fragment key representation. Elsewhere in this repo, a root fragment is represented with a single annotation (i.e., `fid == ftype` via a single-item annotation list), but this branch produces a key where both `fid` and `ftype` are equal. Semantically it is still a root fragment because `fid == ftype` holds.

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.deserialize`, `kfxlib/ion.py:IonAnnotation.is_annotation`, `kfxlib/yj_container.py:YJFragmentKey`.

### 4.6 Mapping an entity to a fragment (`YJFragment`)

After decoding, the entity is returned as:

- `YJFragment(fid=None, ftype=ftype, value=value)` if `fid == "$348"`
- otherwise `YJFragment(fid=fid, ftype=ftype, value=value)`

This is how the special `$348` placeholder is used in this implementation to indicate “this entity has no distinct id; treat it as a root fragment”.

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.deserialize`.

### 4.7 Writer behavior (how this repo emits `ENTY` records)

`KfxContainerEntity.serialize` emits:

- `signature = ENTY`
- `version = 1`
- placeholder `header_len`
- `entity_info` Ion struct containing exactly `$410 = 0` and `$411 = 0`
- then patches `header_len` to the current byte count
- then writes payload bytes:
   - for `$417`/`$418`: raw bytes (must be an `IonBLOB`, otherwise it raises)
   - otherwise: Ion Binary stream of a single value

Derived from: `kfxlib/kfx_container.py:KfxContainerEntity.serialize`.

### 4.8 Which fragments become `ENTY` records when serializing a container

When building a `CONT` file, the serializer excludes container-level fragments from the entity directory:

- It does **not** emit entities for fragment types in `CONTAINER_FRAGMENT_TYPES = ["$270", "$593", "$ion_symbol_table", "$419"]`,
- except that it *does* emit `$419` as an entity anyway (explicit special case).

In other words, `$270`/`$593`/`$ion_symbol_table` live in the outer fragment list but are not written as entities, while `$419` is a required fragment but is written as an entity.

Derived from: `kfxlib/kfx_container.py:KfxContainer.serialize`, `kfxlib/yj_container.py:CONTAINER_FRAGMENT_TYPES`.

### 4.9 How `fid` is represented in the entity directory when serializing

When serializing the entity directory, the `id_idnum` is set to:

- the symbol-id of `$348` if the fragment is a single-annotation fragment (`fragment.is_single()`),
- otherwise the symbol-id of `fragment.fid`.

This is the inverse of the deserializer behavior that maps `fid == "$348"` back to a root fragment (by passing `fid=None` to `YJFragment`).

Derived from: `kfxlib/kfx_container.py:KfxContainer.serialize`, `kfxlib/ion.py:IonAnnotation.is_single`.

---

## 5. Fragment model (“YJ fragments”)

This project represents the decoded contents of a book as a flat list of fragments (`YJFragmentList`).
Each fragment is conceptually:

- `ftype`: a symbol identifying the kind of fragment (example: `$260` = section)
- `fid`: a symbol identifying the instance (example: a particular section name). For “root” fragments, `fid == ftype`.
- `value`: if `ftype` is in `RAW_FRAGMENT_TYPES` (`$417`, `$418`), the entity payload is stored as raw bytes (`IonBLOB`); otherwise it is parsed as a single Ion value

Derived from: `kfxlib/yj_container.py:YJFragment`, `YJFragmentKey`, `YJFragmentList`.

### 5.1 Root fragments vs non-root fragments

The code treats certain fragment types as “root fragments” whose canonical form uses `fid == ftype`.
The lists of required/allowed/root/singleton types are enumerated in `kfxlib/yj_container.py`.

Derived from: `kfxlib/yj_container.py:ROOT_FRAGMENT_TYPES`, `SINGLETON_FRAGMENT_TYPES`, `REQUIRED_BOOK_FRAGMENT_TYPES`, `ALLOWED_BOOK_FRAGMENT_TYPES`.

---

## 6. The `$419` container_entity_map fragment

The converter uses a “container entity map” fragment to describe which fragments belong to which container(s), and (when present as `$419.$253`) dependencies between fragments/resources.

### 6.1 Structure

`$419` value is an Ion struct with fields:
- `$252` (`container_list`): list of container entries
- `$253` (optional): entity dependency list (see below). It is emitted only if the computed/retained dependency list is non-empty.

Each entry in `$252` is a struct:
- `$155` (`id`): container_id (string/symbol)
- `$181` (`contains`): list of fragment IDs (fids) present in that container

If `$253` is present, it is a list of structs describing resource dependencies inferred by the builder:
- `$155` (`id`): fragment id which depends on resources
- `$254`: list of mandatory dependent ids
- `$255`: list of optional dependent ids

Exact presence rules for `$253` in this repo:

- If `rebuild_container_entity_map(..., entity_dependencies=...)` is called with a non-empty dependency list, `$253` is set to that list.
- If called with `entity_dependencies=None`, it retains any existing `$253` value from a prior `$419` fragment.
- If the resulting dependency list is empty/falsey, `$253` is omitted.

Derived from: `kfxlib/yj_structure.py:BookStructure.rebuild_container_entity_map`, `determine_entity_dependencies`.

---

## 7. “Core” fragment types and schemas (inferred)

This section is intentionally conservative: it only states fields that are directly accessed/validated in this codebase.
Many fragments contain additional fields that this converter does not interpret; those will show up as “extra keys” in validation logs.

### 7.1 `$270` (Container)

The converter reconstructs a normalized `$270` fragment during container parsing with fields:
- `$409` container id
- `$412` chunk size
- `$410` compression type
- `$411` DRM scheme
- `$587` / `$588`: generator application/package versions (strings)
- `$161`: container format label ("KFX main" / "KFX metadata" / "KFX attachable" / "KPF")
- `version`: container version number (1/2)
- `$181`: list of `[type_idnum, id_idnum]` pairs for entities in this container

Derived from: `kfxlib/kfx_container.py:KfxContainer.deserialize` (creates `self.container_info` fragment).

### 7.2 Reading order lists (found in `$538` and `$258`)

The code expects a `reading_orders` list under `$169` with each entry having:
- `$178`: reading order name
- `$170`: list of section IDs in order

Exact source/precedence rules:

- If fragment `$538` (document_data) exists, reading orders are taken from `$538.$169`.
- If `$538` exists *and* `$258` exists, `$258.$169` is cross-checked for equality; mismatches are logged.
- If `$538` does **not** exist, reading orders are taken from `$258.$169` (if present).

Derived from: `kfxlib/yj_structure.py:BookStructure.check_consistency` (reading order validation).

### 7.3 `$258` (Metadata) and `$490` (BookMetadata)

**IMPORTANT**: Despite their similar names, these two fragment types serve different purposes:

**`$258` (metadata)** - Contains document structure information:
- `$169` (reading_orders): List of reading order definitions with section references
- May also contain some legacy metadata fields (title, author, etc.) in older KFX files

**`$490` (book_metadata)** - Contains categorised metadata about the book:
- `$491` (categorised_metadata): List of category entries, each containing:
  - `$495` (category): Category name string (e.g., "kindle_title_metadata", "kindle_audit_metadata")
  - `$258` (metadata): List of key-value entries within that category
    - Each entry has `$492` (key) and `$307` (value)

Common `$490` categories in modern KFX:
- `kindle_title_metadata`: title, author, ASIN, content_id, asset_id, book_id, language, publisher, description, cover_image, cde_content_type, is_sample, override_kindle_font
- `kindle_audit_metadata`: creator_version, file_creator (converter info)
- `kindle_ebook_metadata`: selection, nested_span (capability flags)
- `kindle_capability_metadata`: (usually empty)

The converter reads title/author/etc from either:
- `$490` → `$491` (categorised_metadata list) → category `kindle_title_metadata` → `$258` list of key/value structs (`$492` key, `$307` value)
- or `$258` directly, where certain keys are known (e.g. `$153` title, `$222` author, `$224` ASIN, `$10` language, …)

**Note on requirement rules**: If `$490` is present, `$258` is not strictly required (and vice versa). Modern KFX files typically include both, with `$258` containing reading orders and `$490` containing book metadata.

Derived from: `kfxlib/yj_metadata.py:BookMetadata.get_yj_metadata_from_book`, `kfxlib/yj_structure.py:METADATA_SYMBOLS`, `convert/kfx/frag_metadata.go:BuildMetadata`, `convert/kfx/frag_metadata.go:BuildBookMetadata`.

### 7.4 `$538` (DocumentData)

This is treated as a root fragment. If present, `reading_orders` are read as `document_data.value.get("$169", [])`.
If both `$538` and `$258` exist and their `$169` lists differ, an error is logged; if `$538` is missing then `$258.$169` is used instead.

Derived from: `kfxlib/yj_structure.py:BookStructure.check_consistency` (document_data / metadata reading_orders comparison).

### 7.5 `$164` (External resource descriptor) + `$417` (RawMedia) / `$418` (RawFont)

The resource descriptor fragment `$164` is used to locate and validate resource bytes stored separately in raw entities:
- `$175` (`resource_name`): resource identifier - **must be a symbol**, not a string (KPV requirement)
- `$165` (`location`): key used to look up the raw resource entity (`$417`/`$418`)
- `$161` (`format`): file format symbol (e.g. `$285` jpg, `$284` png, `$565` pdf, `$548` jxr)
- `$162` (`mime_type`): MIME type string (use `"image/jpg"` not `"image/jpeg"` for JPEG images)
- `$422`/`$423` (resource_width/resource_height): image dimensions in pixels
- `$636`: tiling structure (list of rows of tile locations)
- `$564`: PDF page number base (0-based, code uses +1 for display)
- `$797`: overlapped tiles flag/metadata (presence indicates overlap)

**Important**: The `$175` field must be encoded as an Ion **symbol**, not a string. KPV validates this and may fail to display images if `$175` is a string. Similarly, `$162` should use `"image/jpg"` (not `"image/jpeg"`) to match KPV's expected format.

Raw bytes are stored as separate fragments:
- `$417` (bcRawMedia) with `fid == location` and value = raw bytes
- `$418` (bcRawFont) similarly for fonts

Derived from: `kfxlib/yj_structure.py:BookStructure.check_consistency` (resource scan), `kfxlib/unpack_container.py:ZipUnpackContainer.deserialize/serialize`, `kfxlib/kfx_container.py:KfxContainerEntity.deserialize`, `convert/kfx/frag_resource.go`.

### 7.5.1 `$260` (Section) and `$259` (Storyline) fragments

**`$260` (section)** - Represents a section (chapter/page) in the book:
- `$174` (section_name): section identifier matching the fragment id
- `$141` (page_templates): list of page template entries

**Page template entry structure** (entries in `$141`):

Per Kindle Previewer (KPV) reference format, page templates use a minimal 3-field structure:
- `$155` (id): EID for the page template
- `$159` (type): content type, always `$269` (text) for standard book sections
- `$176` (story_name): reference to the storyline fragment containing content

**NOTE**: Earlier implementations used a more complex structure with `$270` (container type), `$140` (float), `$156` (layout), `$56`/`$57` (dimensions). This caused rendering issues where only the first page of content would display. The KPV-compatible format uses only the 3 fields above with text type (`$269`).

**`$259` (storyline)** - Contains the actual content for a section:
- `$176` (story_name): storyline identifier matching the fragment id
- `$146` (content_list): list of content entries

**Content entry structure** (entries in `$146`):
- `$155` (id): unique EID for this content element
- `$159` (type): content type symbol (`$269`=text, `$271`=image, `$270`=container)
- `$157` (style): optional style name reference
- `$145` (content): for text, a struct with `name` (content fragment reference) and `$403` (array index/offset within the content_list)
- `$175` (resource_name): for images, external resource fragment id as **symbol** (not string)
- `$584` (alt_text): for images, accessibility text (only included when non-empty, per KPV parity)
- `$142` (style_events): optional inline formatting events
- `$790` (yj.semantics.heading_level): for headings, level 1-6 (KPV parity)

**Style event structure** (entries in `$142`):
- `$143` (offset): start offset within text (**character/rune offset**, not byte offset)
- `$144` (length): span length in characters/runes
- `$157` (style): style name reference
- `$179` (link_target): optional link anchor reference
- `$616` (yj.display): for footnote links, set to `$617` (yj.note) (KPV parity)

**Important**: Offsets and lengths in style events (`$143`, `$144`) are measured in **Unicode code points (characters/runes)**, not bytes. For text containing multi-byte characters (e.g., Cyrillic, CJK), the character offset will differ from the byte offset. For example, the Russian text "Автор" is 5 characters but 10 bytes in UTF-8.

Derived from: `convert/kfx/frag_storyline.go`, KPV reference files.

### 7.5.2 Cover section structure

Cover images require special handling in KFX to enable full-screen scaling. Unlike regular text sections, the cover uses a **container type** (`$270`) page template with explicit dimensions.

**Cover section (`$260`) page template structure**:
- `$140` (float): alignment, typically `$320` (center)
- `$155` (id): unique EID for the page template
- `$156` (layout): scaling mode, typically `$326` (scale_fit)
- `$159` (type): **must be `$270` (container)**, not `$269` (text)
- `$176` (story_name): reference to the cover storyline
- `$66` (container_width): image width in pixels
- `$67` (container_height): image height in pixels

**Cover storyline (`$259`) content entry**:
- `$155` (id): unique EID for the image content
- `$159` (type): `$271` (image)
- `$175` (resource_name): external resource fragment id (as symbol, not string)
- `$157` (style): minimal style with `font-size: 1rem`, `line-height: 1.0101lh`
- `$584` (alt_text): only included when non-empty

**Critical**: For the cover image to scale properly (fill the screen without white borders), it **must** be registered in the landmarks navigation container with type `$233` (cover_page). Without this landmark entry, KPV treats the cover as regular content and does not apply full-screen scaling.

**External resource (`$164`) for cover**:
- `$161` (format): format symbol (`$285`=jpg, `$284`=png, `$286`=gif)
- `$162` (mime_type): MIME type string (use `"image/jpg"` not `"image/jpeg"`)
- `$165` (location): resource path string (e.g., `"resource/rsrc1"`)
- `$175` (resource_name): resource name as **symbol** (not string)
- `$422` (resource_width): image width in pixels
- `$423` (resource_height): image height in pixels

Derived from: `convert/kfx/frag_storyline.go:NewCoverPageTemplateEntry`, `convert/kfx/frag_resource.go`, KPV reference files.

### 7.6 Position and location mapping fragments

This codebase models Kindle positions using these concepts:

- **EID**: an “element id” / location id that identifies a content stream.
   - In structs, EID is carried in either `$155` or `$598`.
- **EID offset**: an integer offset within the EID stream, carried in `$143`.
- **PID**: a global “position id” counter that advances across sections and content.
- **LOC**: a “location” is effectively a sampled PID; in this codebase $\text{LOC} = \lfloor \text{PID} / 110 \rfloor$ (110 positions per location).

Derived from: `kfxlib/yj_position_location.py:KFX_POSITIONS_PER_LOCATION`, `kfxlib/yj_to_epub_navigation.py:get_location_id/get_position`.

#### 7.6.1 Position tuple encoding (used by anchors/nav targets)

Many structures reference a “position” as a struct containing:

- `$155` or `$598`: EID
- optional `$143`: EID offset (default `0`)

In the EPUB conversion pipeline, this is normalized into a tuple `(eid, offset)` by:

- `get_location_id(struct)` which pops `$155` first, else `$598`
- `get_position(position_struct)` which additionally pops `$143` and validates emptiness

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.get_location_id`, `get_position`.

#### 7.6.2 `$264` position_map (EID → section membership)

When present (non-dictionary/non-KPF-prepub path), `$264` is a list of Ion structs, one per section:

- `$174`: section id (the `$260` fragment id)
- `$181`: list of EIDs belonging to that section

**EID list encoding in `$181`**:

Kindle Previewer (KPV) outputs a **flat list of scalar EIDs** (e.g., `[1, 2, 3, 4, 5]`).

Some readers also support compressed `[base_eid, count]` pairs that expand to `base_eid..base_eid+count-1`, but this format is **not recommended** for compatibility with KPV validation. This converter generates flat EID lists matching KPV behavior.

This map is used for validation (detect extra/missing sections and mismatched EIDs).

Derived from: `kfxlib/yj_position_location.py:BookPosLoc.collect_position_map_info`, `convert/kfx/frag_positionmaps.go`.

#### 7.6.3 `$265` position_id_map and sectionized position maps

This converter supports two layouts for `$265`:

1. **Flat map**: `$265.value` is an Ion list that is parsed as a single “SPIM-like” stream (see below).
2. **Sectionized map**: `$265.value` is an Ion struct with `$181` being a list of per-section descriptors:
    - `$174`: section id
    - `$184`: section start PID
    - `$144`: section length (PID count)

When sectionized, for each section descriptor in `$265.$181` the code looks up a `$609` fragment (`section_position_id_map`) with `fid == section_name`; if it is missing, an error is logged and that section is skipped.

Derived from: `kfxlib/yj_position_location.py:BookPosLoc.collect_position_map_info` (handling of `$265` and `$609`).

#### 7.6.4 `$609` section_position_id_map (“SPIM”) entry encoding

The `$609` fragment value is an Ion struct with:

- `$174`: section id
- `$181`: a list of “position id entries” describing a monotone mapping from PID → (EID, EID offset)

Entries inside `$181` take one of three forms:

- **List form**: `[next_pid, next_eid]` or `[next_pid, next_eid, next_eid_offset]`
- **Int form**: `next_pid` only; implies `next_eid += 1` and offset `0`
- **Struct form**: `{ $184: next_pid, $185: next_eid, optional $143: next_eid_offset }`

Invariants checked by this implementation:

- The final entry must end with `eid == 0` and `eid_offset == 0`.
- Offset consistency rule (exact behavior):
   - Let `eid_start_pid` be the PID where an EID first appears.
   - Computed `eid_offset` is `pid - eid_start_pid`.
   - If `eid_offset != (pid - eid_start_pid)`:
      - If any of these conditions are true:
         - the book has illustrated-layout conditional page templates, OR
         - the book declares the `yj_mathml` feature, OR
         - `has_non_image_render_inline()` returns true,
         then the converter enforces a weaker invariant: for each EID, offsets must be strictly increasing over time (`eid_offset > previous_offset_for_eid`). Violations are logged as errors.
      - Otherwise, mismatches are only logged as warnings.

Related format-capability cross-checks performed by this code:

- `format_capabilities.kfxgen.positionMaps == 2` must match presence of a sectionized SPIM (`$265` as struct + per-section `$609`).
- `format_capabilities.kfxgen.pidMapWithOffset == 1` must match whether any non-zero EID offset was observed while parsing the SPIM stream(s).

Derived from: `kfxlib/yj_position_location.py:BookPosLoc.collect_position_map_info` (`process_spim(...)`).

#### 7.6.5 Dictionary / KPF-prepub variant: `$611` + `-spm` section maps

For dictionaries and KPF-prepub, this codebase expects a different mapping shape:

- `$611` (yj.section_pid_count_map): a root fragment whose value has `$181` list entries with:
   - `$174`: section id
   - `$144`: PID count for that section
- For each section, the code looks up a `$609` fragment whose `fid` is `"<section>-spm"`; if missing it logs an error and continues.
   - The SPIM is interpreted as **one-based PIDs** (`one_based_pid=True`) and EIDs are not forced to ints (`int_eid=False`).

Derived from: `kfxlib/yj_position_location.py:BookPosLoc.collect_position_map_info` (dictionary/KPF-prepub branch).

#### 7.6.6 `$550` location_map and `$621` yj.location_pid_map

This converter consumes up to two “location” representations, with a defined precedence:

- `$550` (location_map): validated to be a list of length 1 containing a struct whose keys are a subset of `{ $182, $178 }` (otherwise logs `Bad location_map`).
   - `$182` is a list of structs each containing:
      - `$155`: EID
      - optional `$143`: EID offset
   - This is interpreted as an ordered list of “locations”, each resolvable to a PID via the position maps.

- `$621` (yj.location_pid_map): validated to be a list of length 1 containing a struct whose keys are a subset of `{ $182, $178 }` (otherwise logs `Bad yj.location_pid_map`).
   - `$182` is a list of integer PIDs.
   - If `$550` was successfully processed first, `$621` is used only to cross-check that its PIDs match the PIDs derived from `$550`.
   - If `$550` is missing (or produced no location list), `$621` is used as the primary list and is inverted through the position maps to recover `(eid, offset)`.

Derived from: `kfxlib/yj_position_location.py:BookPosLoc.collect_location_map_info`.

#### 7.6.7 How positions are used to place anchors in generated EPUB

The EPUB conversion pipeline maintains a map:

- `position_anchors[eid][offset] -> [anchor_name...]`

Anchors are registered from `$266` (and from navigation/page list generation). During HTML generation:

- Each content element has a `location_id` extracted from its content struct (`$155` or `$598`).
- The converter calls `process_position(eid, 0, elem)` and later tries to resolve additional offsets by splitting text runs (`locate_offset(...)`).
- When a registered `(eid, offset)` is reached, the first anchor at that position is assigned as the element `id`, and all anchors at that position are associated with that element.

Derived from: `kfxlib/yj_to_epub_navigation.py:register_anchor/process_position/fixup_anchors_and_hrefs`, `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content` (position handling and `locate_offset`).

#### 7.6.8 Consumed keys summary (positions/locations)

This is a compact summary of which fields this repo reads/consumes for the position/location fragments. It is intended as a “wire checklist” when implementing a compatible decoder.

Derived from: `kfxlib/yj_position_location.py:BookPosLoc.collect_position_map_info`, `verify_position_info`, `collect_location_map_info`.

| Fragment | Top-level shape (validated) | Fields used / interpreted | Notes / strictness |
|---:|---|---|---|
| `$264` | `IonList` of `IonStruct` | Per entry: `$174` (section id), `$181` (EID list); EID list supports scalar EIDs or `[base, count]` ranges | Validation-oriented; logs extra/missing sections and EID mismatches |
| `$265` | `IonList` **or** `IonStruct` | If list: treated as one SPIM-like stream. If struct: `$181` list of section descriptors (`$174`, `$184` section start PID, `$144` section length) | When sectionized, expects corresponding `$609` per section |
| `$609` | `IonStruct` | `$174` (section id), `$181` (SPIM entry list). Entry types: list `[pid,eid,(offset)]`, int (pid with implied eid++), or struct `$184/$185/optional $143` | Enforces terminal `eid==0` and `eid_offset==0`; checks monotonicity and offset consistency (with feature-based relaxations) |
| `$611` | `IonStruct` | `$181` list of `{ $174: section id, $144: section pid count }` | Dictionary/KPF-prepub mode expects `$609` fragments keyed by `"<section>-spm"` |
| `$550` | `IonList` of length 1 with `IonStruct` | Struct keys `$182` and `$178` only; `$182` list entries are structs with `$155` (EID) and optional `$143` (offset) | If present, used to compute and validate LOC→PID mapping via position maps |
| `$621` | `IonList` of length 1 with `IonStruct` | Struct keys `$182` and `$178` only; `$182` is list of integer PIDs | If `$550` exists, PIDs are cross-checked; else used to infer EID/offset by inverse lookup |

### 7.7 Navigation fragments (`$389`, `$391`, `$394`, `$390`)

This repository consumes navigation primarily to generate:

- NCX/TOC entries
- EPUB guide/landmarks
- Page-map entries
- Anchor targets in HTML parts

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_anchors`, `process_navigation`.

#### 7.7.1 `$266` (Anchor) fragments

Anchor fragments are collected first. Each `$266` entry is validated and then interpreted as either:

- External URI anchor:
   - `$186`: URI string (special-cased: `"http://"` and `"https://"` are treated as empty)

- Position anchor:
   - `$183`: a position struct (see §7.6.1); the converter registers the anchor at that position

Other observed keys:

- `$597` is tolerated and discarded.

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_anchors`.

#### 7.7.2 `$390` section_navigation (nav containers per section)

`$390` is consumed as a list of structs that associate nav containers with a section:

- `$174`: section id
- `$392`: list of nav-container ids (these are ids of `$391` fragments)

This is used to build `nav_container_section[nav_container_id] = section_id` which later affects some nav behaviors (e.g. certain TOC/landmark mappings).

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_navigation` (first loop over `$390`).

#### 7.7.3 `$389` book_navigation (per reading order)

`$389` is consumed as a list of per-reading-order navigation records:

- `$178`: reading order name
- `$392`: list of nav-container ids

The converter processes reading orders in order, matches `$389.$178`, and then loads each referenced `$391` nav-container fragment.

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_navigation`.

#### 7.7.4 `$391` nav_container schema

Nav containers are stored as `$391` fragments addressed by id.
Each container is an Ion struct with keys:

- `$239`: nav_container_name override.
   - Exact behavior: `$239` replaces the *semantic name* (`nav_container_name`) used for downstream processing (e.g. section association via `$390`, approximate-page-list detection, and log messages).
   - It does **not** affect fragment retrieval: the `$391` fragment is still fetched using the id referenced from `$392` / `imports`.
- `$235`: nav_container_type
- One of:
   - `imports`: list of other `$391` ids to process (recursive include), or
   - `$247`: list of nav-unit ids (ids of `$393` fragments)

Recognized nav_container_type values (this converter logs an error on others):

- `$212`: TOC
- `$236`: landmarks/guide
- `$237`: page list
- `$213`, `$214`: additional nav modes used by some books
- `$798`: headings (special handling to infer heading levels)

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_nav_container`.

#### 7.7.5 `$393` nav_unit schema (recursive)

Each nav unit is a `$393` fragment; the converter treats its value as an Ion struct with:

- `$241` (representation struct, optional):
   - `$244`: label string
   - `$245`: icon resource id (`$164`), used to render an icon
   - `$146`: description as a content list (rendered to text)
- `$154` (optional): overrides/sets description string

- `$240` (optional): unit name; defaults to the label or `"page_list_entry"` depending on context

- `$246` (optional): target position struct (see §7.6.1). If missing, the unit becomes a pure container for children.

- Children:
   - `$247`: list of nested nav-unit ids
   - `$248`: list of “entry_set” structs, each containing:
      - `$247`: list of nested nav-unit ids
      - `$215`: orientation discriminator; used to include/exclude nested entries based on orientation lock

- `$238` (optional): “landmark_type” or heading-level discriminator depending on nav_container_type

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_nav_unit`, `get_representation`.

#### 7.7.6 How nav targets become EPUB anchors

During navigation processing:

- When a nav unit has `$246`, the converter registers an anchor for its target position.
   - Temporary HTML uses `href="anchor:<name>"` URIs.
   - After HTML generation, `fixup_anchors_and_hrefs()` resolves anchor URIs into file-relative `href` links.

Anchor registration is position-based:

- `register_anchor(name, (eid, offset))` records the mapping and returns the generated HTML `id`.
- The actual element is assigned the `id` when the content generator reaches that `(eid, offset)`.

Derived from: `kfxlib/yj_to_epub_navigation.py:register_anchor/process_position/fixup_anchors_and_hrefs`, `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content`.

#### 7.7.7 Landmarks/guide and page list special cases

Landmarks (`nav_type == $236`):

- Uses `$238` to pick a `guide_type` (e.g. cover/text/toc), falling back to the raw value if unknown.

##### Landmarks container structure

The landmarks container is included in `$389` (book_navigation) alongside TOC and page list:

```
{$235: symbol($236), $247: [landmark_entries...]}
```

Each landmark entry has the form:
- `$238` (landmark_type): type symbol identifying the landmark purpose
- `$241` (representation): struct containing `$244` (label) with display text
- `$246` (target_position): struct with `$143: 0` (offset) and `$155: eid` (target EID)

**Standard landmark types**:
- `$233` (cover_page): Cover image - **required for proper cover scaling**
- `$212` (toc): Table of Contents page
- `$396` (srl): Start Reading Location - where reading begins after cover/frontmatter

**Important**: The cover landmark (`$238: symbol($233)`) is **critical** for enabling full-screen cover display. Without this landmark, KPV does not recognize the cover section as special and renders it with standard margins/borders instead of scaling to fill the screen. The landmark must point to the cover section's page template EID.

Example landmarks container:
```
{
  $235: symbol($236),  // nav_type = landmarks
  $247: [
    {$238: symbol($233), $241: {$244: "cover-nav-unit"}, $246: {$143: 0, $155: 1000}},
    {$238: symbol($212), $241: {$244: "Table of Contents"}, $246: {$143: 0, $155: 1867}},
    {$238: symbol($396), $241: {$244: "Start"}, $246: {$143: 0, $155: 1003}}
  ]
}
```

Derived from: `convert/kfx/frag_storyline.go:buildLandmarksContainer`, `convert/kfx/values.go:NewLandmarkEntry`, KPV reference files.

Page list (`nav_type == $237`):

- Expects `$240` to be `"page_list_entry"`.
- Uses the label as the page number and registers anchors named like `page_<label>` (prefixed by reading order name when multiple reading orders exist).
- Exact approximate-page suppression rule:
   - If `nav_container_name == APPROXIMATE_PAGE_LIST` and `not (KEEP_APPROX_PG_NUMS or DEBUG_PAGES)` then page-list entries with a non-empty label do **not** produce anchors/pagemap entries.
   - The first time this happens, it logs `"Removing approximate page numbers previously produced by KFX Output"` and sets an internal flag so the warning is emitted only once.

##### APPROXIMATE_PAGE_LIST Structure

When generating approximate page numbers, the page list container uses a special `nav_container_name`:

- Structure: `{$235: symbol($237), $239: symbol(APPROXIMATE_PAGE_LIST), $247: [entries...]}`
- The `$239` field contains `APPROXIMATE_PAGE_LIST` as a local symbol (not a known YJ_symbols entry)
- Each page entry has the form: `{$241: {$244: "page_number"}, $246: {$143: offset, $155: eid}}`
  - `$241.$244`: page number as string label (e.g., "1", "2", "3")
  - `$246.$143`: character offset within the target element's text (in runes)
  - `$246.$155`: target EID (element ID) where this page starts

Page positions are typically calculated by accumulating text length across content elements and creating page boundaries at regular intervals (e.g., every ~1000-2500 characters).

Derived from: `kfxlib/yj_to_epub_navigation.py:process_nav_container`.

#### 7.7.8 Heading navigation (`nav_type == $798`)

For heading navigation, `$238` is interpreted as a heading-level discriminator:

- `$799..$804` map to heading levels 1..6

The converter carries this heading level as a style attribute (`-kfx-heading-level`) on the element that receives the anchor.

Derived from: `kfxlib/yj_to_epub_navigation.py:process_nav_unit` (heading-level mapping) and `process_position` (sets `-kfx-heading-level`).

#### 7.7.9 `$394` conditional_nav_group_unit

This converter expects `$394` fragments to be absent after processing; remaining data is treated as unexpected.

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_navigation` (`check_empty(self.book_data.pop("$394", {}), ...)`).

#### 7.7.10 Consumed keys summary (navigation)

This is a compact summary of the fields this repo consumes for navigation-related fragments when generating EPUB navigation.

Derived from: `kfxlib/yj_to_epub_navigation.py:KFX_EPUB_Navigation.process_anchors`, `process_navigation`, `process_nav_container`, `process_nav_unit`.

| Fragment / value | Shape (validated) | Fields used / popped | Notes / strictness |
|---:|---|---|---|
| `$266` anchor | `IonStruct` per anchor id | Either `$186` (external URI) **or** `$183` (position struct); optional `$597` discarded | Remaining keys after processing are treated as unexpected (`check_empty`) |
| `$390` section_navigation | `IonList` of `IonStruct` | Each entry: `$174` (section id), `$392` (list of `$391` ids) | Entry is checked to be empty after popping these keys |
| `$389` book_navigation | `IonList` of `IonStruct` | Each entry: `$178` reading order name, `$392` list of `$391` ids | Extra keys after processing are treated as unexpected |
| `$391` nav_container | `IonStruct` | Pops: `mkfx_id` (ignored), `$239` name override (optional), `$235` type, and then either `imports` (recursive includes) or `$247` list of `$393` ids | Logs error for unknown `$235`; expects no leftover keys |
| `$393` nav_unit | `IonStruct` | Pops: `mkfx_id` (ignored), `$241` representation (optional), `$154` description (optional), `$240` name (optional), `$246` target position (optional), `$238` landmark/heading discriminator (optional), `$247` children, `$248` entry_sets | Entry_sets: pop `$247` and `$215` (orientation) | Recursive; treats missing `$246` as “container-only”; expects no leftover keys at each nesting level |
| `$394` conditional_nav_group_unit | mapping | The `$394` table is popped and then `check_empty(...)` is applied | Any remaining keys are logged as unexpected and discarded |

### 7.8 Styles: `$157` (KFX style fragments) + inline property blobs

This converter treats “styles” as **named property bundles** stored in `$157` fragments and referenced from content nodes/events.
The on-wire representation is Ion structs/lists keyed by `$NNN` symbol ids (see [symdict.md](symdict.md)).

Derived from: `kfxlib/yj_to_epub.py:KFX_EPUB.__init__`, `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content`, `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.add_kfx_style`, `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.process_content_properties`.

#### 7.8.1 `$157` fragment shape and naming

- Fragment type: `$157`
- Fragment “name field” (used for `check_fragment_name`): `$173`
- In `book_data`, `$157` ends up as a mapping: `style_name -> yj_properties_struct`

Important: this repository uses the **fragment id** (the `fid` from the entity directory) as the dictionary key, not the `$173` field. It then validates consistency.

Derived from: `kfxlib/yj_to_epub.py:FRAGMENT_NAME_SYMBOL` (maps `$157`→`$173`), `kfxlib/yj_to_epub.py:KFX_EPUB.__init__` (iterates `self.book_data.get("$157", {})` and calls `check_fragment_name`).

#### 7.8.2 What is inside a “style” (`yj_properties_struct`)

The value of a `$157` style fragment is an Ion struct containing **YJ property keys** (numeric `$NNN` ids).
These are the same property keys that also appear directly on content nodes and style events; in all cases, the conversion logic is keyed off membership in `YJ_PROPERTY_NAMES`.

- The definitive “this is a style property key” set in this converter is:
   - `YJ_PROPERTY_NAMES = set(YJ_PROPERTY_INFO.keys())`
- Each `$NNN` property id is mapped to a CSS property name (and optional value map) by `YJ_PROPERTY_INFO`.
- The conversion is **not** a simple lookup: `property_value(...)` and `convert_yj_properties(...)` apply multiple special-case rules beyond `YJ_PROPERTY_INFO` name/value-map translation, including:
   - value-map substitution only for `IonSymbol` / `IonBool` (§7.8.2.1)
   - resource-symbol to `url(...)` translation for certain ids and nested shapes (§7.8.2.2)
   - numeric and scalar coercions (color normalization, px-suffix rules, special `$173` handling) (§7.8.2.3)
   - decoding of several composite struct/list shapes (length/color/shadow/transform/polygon, etc.) (§7.8.2.4)
   - post-processing / synthesis of CSS declarations (converter-level normalization) (§7.8.5)

Derived from: `kfxlib/yj_to_epub_properties.py:YJ_PROPERTY_INFO`, `YJ_PROPERTY_NAMES`, `KFX_EPUB_Properties.convert_yj_properties`, `KFX_EPUB_Properties.property_value`.

##### 7.8.2.1 Exact “value map” behavior

If `YJ_PROPERTY_INFO[$NNN].values` exists, this implementation consults it only when the Ion value is:

- `IonSymbol`: if the symbol exists in the map, it substitutes the mapped CSS token; otherwise it logs a warning and falls back to `str(symbol)`.
- `IonBool`: if the boolean exists in the map, it substitutes the mapped CSS token; otherwise it warns and falls back to `str(bool)`.

No value-map conversion is applied for numeric values (`IonInt`/`IonFloat`/`IonDecimal`) or strings (`IonString`).

Derived from: `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.property_value`.

##### 7.8.2.2 Exact “resource reference → url(...)” behavior

These YJ property ids are treated as external-resource references **only when their value is an `IonSymbol`**:

- `$479` and `$528` (both map to CSS `background-image`)
- `$175` (used as a resource reference in several composite shapes)

For those ids, the converter calls `process_external_resource(symbol)` to obtain an extracted filename, then builds a stylesheet-relative URL and wraps it via `css_url(...)`.

Additionally, if a property value is an `IonStruct` containing `$175`, the converter:

- discards optional `$56`/`$57` (width/height-like fields),
- then recurses as if the value were just `$175: <IonSymbol>`.

Derived from: `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.property_value` (branches for `IonSymbol` and `IonStruct` containing `$175`).

##### 7.8.2.3 Exact scalar / numeric coercion rules

- `$173` (style-name): if the value is an `IonSymbol`, it is normalized via `unique_part_of_local_symbol(...)`; if the result is empty, the property is treated as absent (`None`).
- Colors: for property ids in `COLOR_YJ_PROPERTIES`, numeric values are converted through `fix_color_value(...)`. For `$70`, if the alpha channel is zero it forces alpha to `0xFF` before conversion.
- “Numeric → px” default: for most numeric properties, any non-zero numeric value is converted via `adjust_pixel_value(...)` and suffixed with `px`.
   - Exceptions: the implementation explicitly *does not* auto-append `px` for a fixed allowlist of numeric ids (e.g. `$112`, `$13`, `$645`, `$72`, `$125`, `$126`, `$42`, etc.) and for SVG mode.

Derived from: `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.property_value` (numeric branch + exception set).

##### 7.8.2.4 Exact composite (`IonStruct` / `IonList`) decoding rules

When a value is an `IonStruct`, this implementation recognizes these shapes:

- Length struct: `{ $307: magnitude, $306: unit }`.
   - Units are mapped by `YJ_LENGTH_UNITS`.
   - If `FIX_PT_TO_PX` is enabled, some positive `pt` magnitudes are converted to `px`.
   - For `$42` (line-height), if `USE_NORMAL_LINE_HEIGHT` is enabled and the computed value equals `NORMAL_LINE_HEIGHT_EM`, it is normalized to the CSS keyword `normal`.
- Color struct: `{ $19: <numeric color> }`.
- Shadow-like struct: contains `$499` and `$500` (and is permitted to contain `$501/$502/$498` and `$336` inset).
- Transform-origin-like struct: for `$549`, expects `$59` and `$58` (with defaults/validation).
- Rect/quad-like struct: if it contains `$58`, it expects `$58/$61/$60/$59` and emits four values (percent signs stripped) with validation.
- Nested resource struct: contains `$175` (see resource rules above).
- Two-value struct: contains `$131` and/or `$132` and emits `"<val1> <val2>"` (defaulting missing sides to `inherit`).

When a value is an `IonList`, this implementation recognizes these shapes:

- `$650`: polygon shape (`process_polygon`).
- `$646`: collision list (mapped via `COLLISIONS`).
- `$98`: transform list (`process_transform`).
- `$497`: list of shadows (each element processed recursively and joined by `, `).
- `$761`: layout hints (mapped via `LAYOUT_HINT_ELEMENT_NAMES`).
- `$531`: list of values joined by spaces.

Unknown composite shapes are logged as errors and converted to a placeholder `"?"`.

Derived from: `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.property_value`.

#### 7.8.3 How content references styles

The converter looks for a *style name* in three places and performs `$157` expansion when present:

1. **Element-level style reference**: if a content struct contains `$157`, the converter pops it, stringifies it, and immediately expands that style into the same dict before further processing.
    - The converter immediately expands it into inline properties before further processing.

    Derived from: `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content` (`self.add_kfx_style(content, str(content.pop("$157", "")))`).

2. **Inline “style events”**: the converter pops `$142` from the content struct (default `[]`). If the resulting list is non-empty and the content type is not `$269`/`$277`, an error is logged.
    - Each style-event contains:
       - `$143` offset (start position in the text/content stream)
       - `$144` length
       - optional `$157` style name reference
       - optional additional YJ properties (direct overrides)
    - After locating the span of text, the converter expands `$157` into the event dict, then converts remaining properties to CSS and applies them to a generated wrapper element.

    Derived from: `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content` (style_events loop; `add_kfx_style(style_event, style_event.pop("$157", None))`; `add_style(event_elem, self.process_content_properties(style_event), replace=True)`).

##### 7.8.3.1 `$142` style-event wire schema (implementation-derived)

Each element of the `$142` list is treated as an Ion struct/dict; the converter unconditionally pops required keys and will raise if they are missing.

- Required fields:
   - `$143` (int): start offset in **characters/runes** (not bytes)
   - `$144` (int): length in **characters/runes** (the converter raises if `<= 0` when creating the style-span wrapper)

**Character vs Byte Offsets**: KFX style events use Unicode code point (character/rune) offsets, not byte offsets. This distinction is critical for text containing multi-byte UTF-8 characters. For example:
- The Russian word "Автор" (5 Cyrillic characters) occupies 10 bytes in UTF-8
- A style event starting after "Автор\n" would have `$143: 6` (6 characters), not `$143: 11` (11 bytes)
- Implementations must count runes/characters, not bytes, when calculating offsets

- Optional “style reference”:
   - `$157` (symbol/string): a `$157` style fragment id/name to expand into this event

- Optional “link overlay”:
   - `$179` (anchor id): if present, the converter wraps the target span in an `<a>` with `href` pointing at that anchor

- Optional “dropcap” marker:
   - `$125` (int): dropcap_lines; if present, the converter treats the event as a dropcap span and also expects `$144/$143` to be coherent with the dropped characters

- Optional “ruby” annotation payload:
   - `$757` (ruby_name): triggers ruby processing for the span
   - Either:
      - `$758` (ruby_id) for a single ruby run, or
      - `$759` (list of structs), each containing `$758` plus its own `$143/$144` relative offsets/lengths inside the event span

- Optional “model” discriminator:
   - `$604` (symbol): if present and not `$606`, a warning is logged

- Optional additional YJ properties:
   - Any remaining keys that are members of `YJ_PROPERTY_NAMES` are treated as style properties and converted to CSS.
   - Any remaining non-property keys are treated as extra data: they are logged by `check_empty(...)` and then discarded (the dict is cleared).

Important processing order (behavioral semantics):

1. `$143/$144` are consumed to locate/split the affected text span.
2. `$157` is expanded into the event dict using “copy-if-missing” semantics (local keys override style keys).
3. Special keys like `$757` (ruby) and `$179` (link) are handled structurally (wrapper elements) before CSS conversion.
4. Remaining property keys are converted to CSS via `process_content_properties(...)` and applied to the wrapper element.

Derived from: `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content` (style_events loop), `find_or_create_style_event_element`, `add_kfx_style`, and `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.process_content_properties`.

3. **First-line style**: if a content struct contains `$622`, it is treated as a first-line style struct.
   - If `$622` is present, it is popped into `first_line_style`.
   - The converter pops `$173` from `first_line_style` (if present) and uses it as the style name for `add_kfx_style(...)`, then pops `$173` again (ensuring it is removed).
   - It pops `$625` and expects it to be a one-entry struct with `$623: 1`; otherwise it logs an error.
   - It converts remaining properties, prefixes them via `partition(name_prefix="-kfx-firstline", add_prefix=True)`, applies them, then `check_empty(...)` logs and discards leftovers.

    Derived from: `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content` (handling of `$622`).

Also observed:

- `$429` is used as a “backdrop style” reference in certain container/layout content.
   - The converter expands the referenced `$157` style into a temporary dict and expects only a small subset of properties (it explicitly discards `$173`, `$70`, `$72` and errors on leftovers).

Derived from: `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.process_content` (backdrop style handling).

#### 7.8.4 Style merge semantics (how `$157` expansion works)

When a style name is referenced, the converter merges the corresponding `$157` fragment properties into the target struct:

- Only keys that are *not already present* are copied.
- For `IonList` and `IonStruct` values, a deep copy is made before inserting.
- If a referenced style is missing, it logs an error once per missing style name.

This means **local properties always override** the referenced style.

Derived from: `kfxlib/yj_to_epub_content.py:KFX_EPUB_Content.add_kfx_style`.

#### 7.8.5 How properties become CSS (converter behavior)

This is not part of the KFX on-wire format, but it is how this repository interprets the style/property blobs:

- `process_content_properties(content_struct)` extracts all keys in `YJ_PROPERTY_NAMES` from the struct and converts them to CSS declarations.
- CSS is initially attached to elements as inline `style="..."` attributes.
- `fixup_styles_and_classes()` later simplifies styles and moves many inline styles into class-based rules.
- `create_css_files()` emits `styles.css` (and a `reset.css`) into the EPUB.

Additional exact “CSS normalization” behaviors that are easy to miss (these are why some properties are effectively handled differently):

- Duplicate property merges:
   - `-kfx-attrib-epub-type`: values are combined (union) as long as non-`amzn:` values do not conflict.
   - `text-decoration`: values are unioned.
   - Any other property with multiple distinct values is logged as an error; the latest value wins in the final dict.
- Synthesized composite CSS properties:
   - `background-position` is synthesized from `-kfx-background-positionx`/`-kfx-background-positiony` with defaults `50%`.
   - `background-size` is synthesized from `-kfx-background-sizex`/`-kfx-background-sizey` with defaults `auto`.
   - `background-color` is synthesized if either `-kfx-fill-color` or `-kfx-fill-opacity` is present.
   - `text-emphasis-position` is synthesized from `-kfx-text-emphasis-position-horizontal` and `-kfx-text-emphasis-position-vertical`.
   - `orphans`/`widows` are synthesized if `-kfx-keep-lines-together` is present (except that `inherit` leaves the field unset).
- Special-case mapping of `position` values:
   - If the computed CSS property name is `position` and the value is `oeb-page-foot` / `oeb-page-head`, it is remapped to `display` (only in an EPUB2+`EMIT_OEB_PAGE_PROPS` mode); otherwise it is dropped.

Derived from: `kfxlib/yj_to_epub_properties.py:KFX_EPUB_Properties.convert_yj_properties` and `property_value`.

#### 7.8.6 “Unused styles” strictness

The conversion process tracks which `$157` styles were referenced (`used_kfx_styles`). After building the EPUB, it removes used styles from the `$157` table and expects the remainder to be empty; leftovers are treated as unexpected.

Derived from: `kfxlib/yj_to_epub.py:KFX_EPUB.__init__` (pops `$157`, removes `used_kfx_styles`, then `check_empty(kfx_styles, "kfx styles")`).

### 7.9 `$585` content_features (reflow and canonical format)

The `$585` fragment contains feature declarations for content-related capabilities. In reference KFX files, the reflow-* and CanonicalFormat features are stored here rather than in `$593` format_capabilities.

**Fragment structure:**

- Fragment type: `$585`
- Root fragment (fid == ftype)
- Value: Ion struct with `$590` (features) field containing a list of feature entries

**Feature entry structure:**

Each entry in the `$590` list is a struct containing:

- `$492` (key): Feature name string (e.g., "reflow-style", "reflow-section-size", "CanonicalFormat")
- `$586` (namespace): Feature namespace (e.g., "com.amazon.yjconversion", "SDK.Marker")
- `$589` (version_info): Struct containing:
  - `version`: Struct with `$587` (major_version) and `$588` (minor_version) as integers

**Common features:**

| Feature Key | Namespace | Description |
|-------------|-----------|-------------|
| reflow-style | com.amazon.yjconversion | Indicates reflow styling support |
| reflow-section-size | com.amazon.yjconversion | Version relates to max section PID count |
| reflow-language-expansion | com.amazon.yjconversion | Language expansion support |
| CanonicalFormat | SDK.Marker | Indicates canonical format compliance |

**reflow-section-size version calculation:**

The major_version for reflow-section-size is derived from the maximum per-section PID count:
- `version = max(1, ceil(log2(maxSectionPIDCount)) - 11)`

This is used by KFXInput for validation and by Kindle for rendering optimization.

Derived from: Reference KFX files, `convert/kfx/frag_contentfeatures.go`.

---

### 7.10 Length units and KPV conventions

KFX uses a dimension struct `{ $307: magnitude, $306: unit }` for all length values in style properties.

**CRITICAL - Ion Type for $307**: The `$307` (value/magnitude) field **MUST** be encoded as **Ion DecimalType**, not Ion Float or Ion String. Kindle Previewer (KPV) will crash or render incorrectly if `$307` is encoded as any other Ion type. When scanning reference KFX files, all numeric dimension values appear exclusively as Ion Decimal (zero Ion Floats).

Implementation note: Use `ion.MustParseDecimal()` or equivalent to create proper Ion Decimal values. The decimal representation should follow KPV conventions (e.g., `"2.5d-1"` for 0.25, `"1."` for 1.0).

Derived from: Reference KFX analysis, `convert/kfx/frag_style.go:DimensionValue`.

#### 7.10.1 Unit symbols

| Symbol | CSS Unit | Description |
|--------|----------|-------------|
| `$308` (`SymUnitEm`) | `em` | Relative to font size |
| `$310` (`SymUnitLh`) | `lh` | Relative to line-height |
| `$314` (`SymUnitPercent`) | `%` | Percentage |
| `$505` (`SymUnitRem`) | `rem` | Relative to root font size |
| `$309` (`SymUnitPx`) | `px` | Pixels |
| `$311` (`SymUnitPt`) | `pt` | Points |

#### 7.10.2 KPV unit conventions (reverse-engineered)

**CRITICAL**: Kindle Previewer (KPV) uses specific unit types for different CSS properties. Using incorrect units can cause rendering issues (e.g., `text-align: center` not working with percentage font-sizes).

| CSS Property | KPV Unit | Notes |
|--------------|----------|-------|
| `font-size` | `rem` | **NOT `%`**. Using `%` breaks text-align rendering |
| `margin-top` | `lh` | Line-height units for vertical spacing |
| `margin-bottom` | `lh` | Line-height units for vertical spacing |
| `margin-left` | `%` | Percentage for horizontal spacing |
| `margin-right` | `%` | Percentage for horizontal spacing |
| `text-indent` | `%` | Percentage |
| `line-height` | `lh` | Line-height units |

#### 7.10.3 Unit conversion ratios

When converting from CSS `em` units to KPV-preferred units:

| Conversion | Ratio | Example |
|------------|-------|---------|
| `em` → `lh` (vertical) | 1:1 | `1em` → `1lh` |
| `em` → `%` (horizontal) | 1:6.25 | `1em` → `6.25%` |
| `%` → `rem` (font-size) | divide by 100 | `140%` → `1.4rem` |
| `em` → `rem` (font-size) | 1:1 | `1em` → `1rem` |
| `em` → `%` (text-indent) | 1:3.125 | `1em` → `3.125%` |

#### 7.10.4 Zero value omission

KPV does NOT include style properties with zero values. For example, `margin-left: 0` is omitted entirely from the style definition rather than being encoded as `{ $48: { $307: 0, $306: "$314" } }`.

Derived from: Reference KFX comparison, `convert/kfx/css_converter.go:setDimensionProperty`.

#### 7.10.5 Padding properties ($52-$55)

KFX supports individual padding properties for table cells and other block elements:

| Symbol | Property | Notes |
|--------|----------|-------|
| `$52` | `padding_top` | Vertical padding in `lh` units |
| `$53` | `padding_left` | Horizontal padding in `%` |
| `$54` | `padding_bottom` | Vertical padding in `lh` units |
| `$55` | `padding_right` | Horizontal padding in `%` |

These are primarily used for table cell styling. The shorthand `padding` CSS property expands to these four individual properties.

Derived from: Reference KFX analysis, `convert/kfx/css_converter.go:expandBoxShorthand`.

#### 7.10.6 Border properties ($83, $88, $93)

KFX supports border styling for tables and other elements:

| Symbol | Property | Value Type |
|--------|----------|------------|
| `$83` | `border_color` | Packed ARGB integer (see §7.10.7) |
| `$88` | `border_style` | Symbol: `$328` (solid), `$330` (dashed), `$331` (dotted), `$349` (none) |
| `$93` | `border_weight` | Dimension struct with `pt` units |

The CSS `border` shorthand expands to these three properties. Border style values:
- `$328` - solid
- `$330` - dashed  
- `$331` - dotted
- `$349` - none

Derived from: Reference KFX analysis, `convert/kfx/css_converter.go:expandBorderShorthand`.

#### 7.10.7 Color format (packed ARGB integer)

**CRITICAL**: KFX stores colors as packed 32-bit ARGB integers, NOT as structs with RGB components.

Format: `0xAARRGGBB` where:
- `AA` = Alpha (always `0xFF` for opaque)
- `RR` = Red (0x00-0xFF)
- `GG` = Green (0x00-0xFF)
- `BB` = Blue (0x00-0xFF)

Examples:
- Black: `0xFF000000` = `4278190080`
- White: `0xFFFFFFFF` = `4294967295`
- Gray (#808080): `0xFF808080` = `4286611584`

This applies to:
- `$83` (border_color)
- `$19` (text_color)
- `$70` (fill_color / background_color)

Derived from: Reference KFX analysis, `convert/kfx/css_values.go:MakeColorValue`.

#### 7.10.8 Orphans/widows NOT used by KPV

**CRITICAL**: KPV-generated KFX files do NOT include orphans (`$131`) or widows (`$132`) properties, despite these symbols existing in the KFX symbol table.

The CSS `page-break-inside: avoid` maps to:
- `$135` (break_inside): `$353` (avoid)

Page break avoidance for keeping content together is handled via:
- `$788` (yj_break_after): `$353` (avoid) or `$383` (auto)
- `$789` (yj_break_before): `$353` (avoid) or `$383` (auto)

If your CSS converter generates `$131`/`$132` as intermediate markers (e.g., from `page-break-after: avoid`), these should be converted to `$788`/`$789` and then deleted before serialization.

Derived from: Reference KFX comparison, `convert/kfx/frag_style.go:convertPageBreaksToYjBreaks`.

#### 7.10.9 Text-align and float symbol mapping

**CRITICAL**: CSS `text-align` property uses **physical direction symbols** (`$59` left, `$61` right), NOT logical direction symbols (`$680` start, `$681` end).

| CSS Value | KFX Symbol | Symbol Name | Notes |
|-----------|------------|-------------|-------|
| `left` | `$59` | `SymLeft` | Physical left alignment |
| `right` | `$61` | `SymRight` | Physical right alignment |
| `center` | `$320` | `SymCenter` | Center alignment |
| `justify` | `$321` | `SymJustify` | Justified text |
| `start` | `$680` | `SymStart` | Logical start (rarely used) |
| `end` | `$681` | `SymEnd` | Logical end (rarely used) |

For `float` property (currently unused in reference KFX files, but supported):
| CSS Value | KFX Symbol | Symbol Name |
|-----------|------------|-------------|
| `left` | `$59` | `SymLeft` |
| `right` | `$61` | `SymRight` |
| `none` | `$349` | `SymNone` |

Reference KFX files from KPV consistently use `$59`/`$61` for left/right alignment, not the logical `$680`/`$681` symbols. Using `$680`/`$681` for text-align may cause rendering inconsistencies.

Derived from: Reference KFX comparison, `convert/kfx/css_values.go:ConvertTextAlign`, `convert/kfx/css_values.go:ConvertFloat`.

---

---

## 8. Symbol dictionary

The full `$NNN` → `b.jad` mapping table is in `symdict.md`.

Derived from: `b.jad` (enum `com.amazon.kaf.c.b`).

---

## 9. YJ book structure (fragments, invariants, references)

This section documents how this repository treats a decoded “book” as a set of YJ fragments, including:
- which fragment types are required/allowed for different book kinds,
- how fragment ids are validated,
- how cross-fragment references are discovered,
- how `$419` (container_entity_map) and `$270` (container) are rebuilt by this repo.

Derived from: `kfxlib/yj_book.py:YJ_Book.decode_book`, `kfxlib/yj_container.py` (fragment model + type sets), `kfxlib/yj_structure.py:BookStructure`.

### 9.1 `YJ_Book` decode flow and container aggregation

High-level decode sequence:

1. Locate input datafiles (single file, directory tree, or zip bundle).
2. For each discovered datafile, pick a container implementation based on signature/extension.
3. Deserialize each container and append its fragments into a single `YJFragmentList`.
4. If the book is KPF-prepub, apply fix-ups.
5. Run consistency checks and reference-graph checks.

Derived from: `kfxlib/yj_book.py:YJ_Book.locate_book_datafiles`, `kfxlib/yj_book.py:YJ_Book.get_container`, `kfxlib/yj_book.py:YJ_Book.decode_book`.

Container selection rules were summarized earlier in §1.1, but the key implementations are:

- Ion text container: `.ion` that is *not* Ion Binary
- Zip-unpack container: ZIP with `book.ion`
- KPF container: `book.kdf` (zip) or `KDF` signature
- KFX `CONT`: `CONT` signature
- DRMION: treated as non-convertible, with a special-case attempt to expand `metadata.kfx`

Derived from: `kfxlib/yj_book.py:YJ_Book.get_container`, `kfxlib/kfx_container.py:KfxContainer`.

### 9.2 Fragment list model used by this repo

This repo stores the book as a flat list of `YJFragment` objects in a `YJFragmentList`.

- `YJFragment` is an `IonAnnotation` whose annotations encode `(fid, ftype)` via `YJFragmentKey`.
- A `YJFragmentKey` is effectively:
   - `[ftype]` for “root fragments”, or
   - `[fid, ftype]` for normal fragments.
- `YJFragmentList` maintains indexes by `ftype` and by full `(fid, ftype)`.
   - `get(ftype=..., fid=...)` returns a single fragment (or raises if multiple exist unless `first=True`).
   - `get_all(ftype)` returns all fragments of that type.

Derived from: `kfxlib/yj_container.py:YJFragmentKey`, `YJFragment`, `YJFragmentList.get/get_all`.

### 9.3 Fragment type sets and how “book kinds” change them

The baseline fragment sets are defined in `kfxlib/yj_container.py`:

- `ROOT_FRAGMENT_TYPES`: types whose canonical “root fragment” form uses `fid == ftype`.
- `SINGLETON_FRAGMENT_TYPES`: root fragment types where this repo expects at most one fragment per book.
- `REQUIRED_BOOK_FRAGMENT_TYPES`: must be present for a “normal” book.
- `ALLOWED_BOOK_FRAGMENT_TYPES`: optional, known fragment types (recognized by this repo but not required).

Derived from: `kfxlib/yj_container.py:ROOT_FRAGMENT_TYPES`, `SINGLETON_FRAGMENT_TYPES`, `REQUIRED_BOOK_FRAGMENT_TYPES`, `ALLOWED_BOOK_FRAGMENT_TYPES`.

The actual required/allowed sets depend on flags and format features computed from metadata and format capabilities:

- Dictionary / Scribe notebook / KPF-prepub:
   - removes `$419`, `$265`, `$264` from required.
- Non-dictionary/non-notebook:
   - removes `$611` from required.
   - if `format_capabilities.kfxgen.positionMaps != 2`, removes `$609` and `$621` from allowed.
- Not KPF-prepub: removes `$610` from allowed.
- Dictionary / notebook / magazine / print replica:
   - removes `$550` from required.
   - and for non-dictionary, discards `$621` from allowed.
- Not magazine: removes `$267` and `$390` from allowed.
- KFX v1: removes `$538` (document_data) from required and discards `$265` (position_id_map).
- Scribe notebook: removes `$389` and `$611` from required and allows `$611`.
- Metadata fragment pair rule:
   - if `$490` present, `$258` is not required; if `$258` present, `$490` is not required.

Derived from: `kfxlib/yj_structure.py:BookStructure.check_consistency` (required/allowed adjustments).

### 9.4 Fragment id validation (`FRAGMENT_ID_KEYS`)

For many fragment types, this repo expects the fragment id (`fid`) to match a field inside the fragment value struct.
The mapping is declared in `FRAGMENT_ID_KEYS`.

Examples:

- `$260` section: id key `$174`
- `$259` storyline: id key `$176`
- `$164` external resource descriptor: id key `$175`
- `$157` style: id key `$173`
- `$417` raw_media and `$418` raw_font: id key `$165` (location)

Special-case id normalization during validation:

- `$609` (section_position_id_map): in dictionaries / notebooks / KPF-prepub, the value-derived id is suffixed with `-spm`.
- `$610`: when the id key is an int, it is normalized to `eidbucket_<n>`.

Derived from: `kfxlib/yj_structure.py:FRAGMENT_ID_KEYS`, `BookStructure.check_consistency` (value_fid extraction), `extract_fragment_id_from_value`.

### 9.5 Multi-container completeness and `$419` cross-check

When a book contains multiple `$270` container fragments, `check_consistency`:

- Builds a mapping `container_id -> $270 fragment`.
- Locates the container id that contains entity type `419` in its `$181` list (this is used to exempt one “extra” container).
- If `$419` exists:
   - For each `$252` entry (`container_list`), compares the fragment ids in `$181` against those listed in the corresponding `$270.$181` (after mapping id numbers back to symbols).
   - Reports missing referenced fragments (excluding `$348`).
   - Reports extra fragments present in a container but absent from `$419`.
   - Reports missing containers (logs that the book is incomplete and suggests combining into a KFX-ZIP).
   - Reports extra containers missing from `$419` (excluding the special “entity_map_container_id”).

Derived from: `kfxlib/yj_structure.py:BookStructure.check_consistency` (container scanning + `$419` validation).

### 9.6 Reading order and navigation consistency (book-level)

At the structure level, this repo treats reading order as authoritative for which `$260` sections should exist.
It validates reading order lists in `$538.$169` (or `$258.$169` fallback) and cross-checks `$389` navigation reading-order names.

Derived from: `kfxlib/yj_structure.py:BookStructure.check_consistency` (reading order + navigation cross-check). See also §7.2 and §7.7.

### 9.7 Reference graph discovery: `walk_fragment`

To determine which fragments are “used” by the book, the repo builds a reference graph starting from:

- all root fragments except `$419` (and `$610` additionally for KPF-prepub)
- fragments with unknown types
- the cover resource id (`metadata.cover_image`), treated as a `$164` reference

It then iteratively walks values of each discovered fragment to find referenced fragment ids.

Reference discovery rules (high-level):

- Traverses Ion values recursively (annotation, list, struct, sexp).
- Treats certain `IonString` fields as symbol references (notably `$165` and `$636`).
- Treats most `IonSymbol` occurrences as potential fragment references based on:
   - `COMMON_FRAGMENT_REFERENCES`
   - `NESTED_FRAGMENT_REFERENCES`
   - `SPECIAL_FRAGMENT_REFERENCES` / `SPECIAL_PARENT_FRAGMENT_REFERENCES`
   - special-case handling for dictionaries and KPF-prepub.
- Also detects “EID” definitions/references for consistency checking.

Derived from: `kfxlib/yj_structure.py:BookStructure.check_fragment_usage`, `walk_fragment`, and the reference-map constants.

### 9.8 Unreferenced fragments, duplicates, and “mixed book” detection

After graph discovery:

- Any referenced fragment key that is missing is reported (missing `$597` is only a warning).
- Any fragment not visited by the graph is treated as unreferenced (error), with some KPF-prepub exceptions.
- Exact duplicate handling rules in this implementation:
   - For fragment types `$270` and `$593`, duplicate keys are silently ignored (the first occurrence wins).
   - For other fragment types (except `$262`/`$387`, which are excluded from the duplicate check), a duplicate key with identical Ion content is logged as a warning (for `$597` it is logged as a known error) and ignored.
   - A duplicate key with different content is logged as an error and causes an exception to be raised:
   - “Book appears to have KFX containers from multiple books. (duplicate fragments)”

Derived from: `kfxlib/yj_structure.py:BookStructure.check_fragment_usage`.

### 9.9 Rebuilding `$270` and `$419` (when requested)

When `check_fragment_usage(rebuild=True)` is used (e.g. to normalize or reserialize):

- `$270` is synthesized iff `rebuild=True` and the book is neither a dictionary nor a Scribe notebook.
   - All existing `$270` fragments are removed from the referenced-fragment set.
   - `container_id` selection order:
     1) if exactly one distinct non-empty `$270.$409` was present, use it;
     2) else use `asset_id` (metadata key `$466`) if present;
     3) else generate a random `CR!` id.
   - `$161` is set to `"KFX main"`.
   - `$587` takes the first non-empty value seen in existing `$270` fragments, else defaults to `"kfxlib-<version>"`.
   - `$588` takes the first non-empty value seen in existing `$270` fragments, else defaults to the empty string.
   - `version` takes the first non-empty value seen in existing `$270` fragments, else defaults to `KfxContainer.VERSION`.
- The fragment list is replaced with *only the referenced fragments* (sorted).
- `$419` is rebuilt as:
   - `$252 = [ { $155: container_id, $181: [entity_ids...] } ]`
   - optional `$253` entity dependency list (see below).

Derived from: `kfxlib/yj_structure.py:BookStructure.check_fragment_usage` (rebuild block), `rebuild_container_entity_map`.

### 9.10 Entity dependencies (`$419.$253`) derivation

If dependency computation is enabled during rebuild, `determine_entity_dependencies(...)`:

- Computes transitive mandatory references for each fragment.
- Then emits dependency records for two specific dependency edges:
   - sections (`$260`) depend on external resources (`$164`)
   - external resources (`$164`) depend on raw media (`$417`)
- For each matching fragment, `$253` entries contain:
   - `$155`: the dependant fragment id
   - `$254`: mandatory dependent ids
   - `$255`: optional dependent ids (only for the `$164`→`$417` edge; derived from optional references)

Derived from: `kfxlib/yj_structure.py:BookStructure.determine_entity_dependencies`.

### 9.11 Symbol table validation and local symbol classification

The repo checks that the `$ion_symbol_table.symbols` list covers all locally used symbols:

- It walks all fragments (excluding container fragments) to find symbol references.
- Compares the set of required local-symbol strings against the original symbols listed in the `$ion_symbol_table` fragment(s).
- `check_symbol_table(rebuild=..., ignore_unused=...)` rebuilds the local symbol list and/or suppresses warnings about unused symbols, depending on its arguments.

It also classifies symbol names (`COMMON`/`DICTIONARY`/`ORIGINAL`/`BASE64`/`SHORT`/`SHARED`) using a set of regexes and whitelists.

Derived from: `kfxlib/yj_structure.py:BookStructure.check_symbol_table`, `classify_symbol`, `create_local_symbol`.
