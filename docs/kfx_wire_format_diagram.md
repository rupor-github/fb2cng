# KFX "CONT" Wire Format Diagram (Simple Book)

This document provides ASCII/pseudo-graphic diagrams of the KFX container format
for a **simple book** - one that doesn't include magazine-specific, dictionary,
notebook, or other optional fragment types.

Based on: [kfxstructure.md](kfxstructure.md)

---

## 1. High-Level File Structure

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         KFX CONT FILE (.kfx, .azw8)                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                      HEADER REGION [0..header_len)                    │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │           Fixed Header (18 bytes)                           │     │  │
│  │   │  ┌──────────────────────────────────────────────────────┐   │     │  │
│  │   │  │ 0x00: "CONT" (4 bytes) - signature                   │   │     │  │
│  │   │  │ 0x04: version (u16 LE) - typically 1 or 2            │   │     │  │
│  │   │  │ 0x06: header_len (u32 LE) - offset to entity region  │   │     │  │
│  │   │  │ 0x0A: container_info_offset (u32 LE)                 │   │     │  │
│  │   │  │ 0x0E: container_info_length (u32 LE)                 │   │     │  │
│  │   │  └──────────────────────────────────────────────────────┘   │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │        Entity Directory (at bcIndexTabOffset)               │     │  │
│  │   │        24 bytes per entry × N entries                       │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │        Doc Symbol Table (at bcDocSymbolOffset)              │     │  │
│  │   │        Ion Binary: $ion_symbol_table annotation             │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │        Format Capabilities (at bcFCapabilitiesOffset)       │     │  │
│  │   │        Ion Binary: $593 annotation (v2 only, optional)      │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │        container_info (Ion Binary struct)                   │     │  │
│  │   │        Points to all above via offset/length pairs          │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │        kfxgen Metadata Blob (JSON-like)                     │     │  │
│  │   │        Contains: SHA1, version info, container id           │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                ENTITY PAYLOAD REGION [header_len..EOF)                │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │  ENTY Record #1                                             │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │  ENTY Record #2                                             │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │  ENTY Record #3                                             │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                            ...                                        │  │
│  │   ┌─────────────────────────────────────────────────────────────┐     │  │
│  │   │  ENTY Record #N                                             │     │  │
│  │   └─────────────────────────────────────────────────────────────┘     │  │
│  │                                                                       │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Fixed Header Detail (18 bytes)

```
Offset   Size    Field                    Value/Description
────────────────────────────────────────────────────────────────
0x00     4       signature                "CONT" (ASCII)
0x04     2       version                  u16 LE (1 or 2)
0x06     4       header_len               u32 LE → start of ENTY region
0x0A     4       container_info_offset    u32 LE → offset within file
0x0E     4       container_info_length    u32 LE → byte length
────────────────────────────────────────────────────────────────
         18 bytes total

┌────┬────┬────┬────┬─────────┬───────────────────┬───────────────────┬───────────────────┐
│ C  │ O  │ N  │ T  │ version │    header_len     │container_info_off │container_info_len │
├────┼────┼────┼────┼────┬────┼────┬────┬────┬────┼────┬────┬────┬────┼────┬────┬────┬────┤
│    │    │    │    │ LO │ HI │ B0 │ B1 │ B2 │ B3 │ B0 │ B1 │ B2 │ B3 │ B0 │ B1 │ B2 │ B3 │
└────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
  0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15   16   17
```

---

## 3. container_info Structure (Ion Binary Struct)

```
container_info (Ion Binary encoded struct)
├── $409 (bcContId)         : string   - Container identifier
├── $410 (bcComprType)      : int      - Compression type (0 = none)
├── $411 (bcDRMScheme)      : int      - DRM scheme (0 = none)
├── $412 (bcChunkSize)      : int      - Chunk size (4096)
├── $413 (bcIndexTabOffset) : int      - Entity directory offset
├── $414 (bcIndexTabLength) : int      - Entity directory length
├── $415 (bcDocSymOffset)   : int      - Doc symbol table offset
├── $416 (bcDocSymLength)   : int      - Doc symbol table length
├── $594 (bcFCapOffset)     : int      - Format capabilities offset (v2)
└── $595 (bcFCapLength)     : int      - Format capabilities length (v2)

                    Header Region Byte Ranges
    ┌─────────────────────────────────────────────────────────────┐
    │                                                             │
0   │ Fixed Header (18 bytes)                                     │
    ├─────────────────────────────────────────────────────────────┤
    │                                                             │
    │ Entity Directory ─────────────────┐                         │
    │   [$413..$413+$414)               │                         │
    │                                   │                         │
    ├───────────────────────────────────┤                         │
    │                                   │                         │
    │ Doc Symbol Table ─────────────────┤                         │
    │   [$415..$415+$416)               │                         │
    │                                   │                         │
    ├───────────────────────────────────┤                         │
    │                                   │                         │
    │ Format Capabilities (v2) ─────────┤                         │
    │   [$594..$594+$595)               │  (optional)             │
    │                                   │                         │
    ├───────────────────────────────────┤                         │
    │                                   │                         │
    │ container_info Ion struct ────────┘                         │
    │   [container_info_offset..+length)                          │
    │                                                             │
    ├─────────────────────────────────────────────────────────────┤
    │                                                             │
    │ kfxgen Metadata Blob (JSON-like)                            │
    │   [container_info_offset+length..header_len)                │
    │                                                             │
    ├─────────────────────────────────────────────────────────────┤
    │ header_len ─────────────────────────────────────────────────│
    └─────────────────────────────────────────────────────────────┘
```

---

## 4. Entity Directory Entry (24 bytes each)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                   Entity Directory Entry (24 bytes)                     │
├───────────┬───────────┬──────────────────────┬──────────────────────────┤
│  id_idnum │ type_idnum│   entity_offset      │     entity_len           │
│  (u32 LE) │ (u32 LE)  │     (u64 LE)         │      (u64 LE)            │
│  4 bytes  │ 4 bytes   │     8 bytes          │      8 bytes             │
├───────────┴───────────┴──────────────────────┴──────────────────────────┤
│ id_idnum    = Symbol ID of fragment id (or $348 for root fragments)     │
│ type_idnum  = Symbol ID of fragment type ($258, $260, $164, etc.)       │
│ entity_offset = Offset RELATIVE TO header_len (not file start!)         │
│ entity_len    = Length of ENTY record in bytes                          │
└─────────────────────────────────────────────────────────────────────────┘

    Entity Directory Layout (multiple entries)
    ┌─────────┬──────────┬─────────────────┬──────────────────┐
    │id_idnum │type_idnum│  entity_offset  │   entity_len     │ Entry 0
    ├─────────┼──────────┼─────────────────┼──────────────────┤
    │id_idnum │type_idnum│  entity_offset  │   entity_len     │ Entry 1
    ├─────────┼──────────┼─────────────────┼──────────────────┤
    │id_idnum │type_idnum│  entity_offset  │   entity_len     │ Entry 2
    ├─────────┼──────────┼─────────────────┼──────────────────┤
    │   ...   │   ...    │      ...        │      ...         │
    └─────────┴──────────┴─────────────────┴──────────────────┘
       4B        4B           8B                8B
```

---

## 5. ENTY Record Format

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           ENTY Record                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    Fixed Header (10+ bytes)                      │   │
│  │  ┌────────────────────────────────────────────────────────────┐  │   │
│  │  │ 0x00: "ENTY" (4 bytes) - signature                         │  │   │
│  │  │ 0x04: version (u16 LE) - must be 1                         │  │   │
│  │  │ 0x06: header_len (u32 LE) - offset to payload data         │  │   │
│  │  └────────────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │              entity_info (Ion Binary struct)                     │   │
│  │              Bytes [0x0A .. header_len)                          │   │
│  │  ┌────────────────────────────────────────────────────────────┐  │   │
│  │  │ $410 (bcComprType) : int (0 = none)                        │  │   │
│  │  │ $411 (bcDRMScheme) : int (0 = none)                        │  │   │
│  │  └────────────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                     entity_data (payload)                        │   │
│  │                     Bytes [header_len .. end)                    │   │
│  │  ┌────────────────────────────────────────────────────────────┐  │   │
│  │  │ For $417/$418: Raw bytes (IonBLOB)                         │  │   │
│  │  │ For all others: Ion Binary single value                    │  │   │
│  │  └────────────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

Byte Layout:
┌────┬────┬────┬────┬─────────┬───────────────────┬─────────────────┬───────────────────┐
│ E  │ N  │ T  │ Y  │ version │    header_len     │  entity_info    │   entity_data     │
├────┼────┼────┼────┼────┬────┼────┬────┬────┬────┼─────────────────┼───────────────────┤
│    │    │    │    │ LO │ HI │ B0 │ B1 │ B2 │ B3 │  (Ion struct)   │     (payload)     │
└────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴─────────────────┴───────────────────┘
  0    1    2    3    4    5    6    7    8    9   10...header_len  header_len...end
```

---

## 6. Ion Binary Value Encoding (Quick Reference)

```
Ion Binary Stream Header: E0 01 00 EA (4 bytes)

Value Descriptor Byte:
┌─────────────────────────────────────────────────────────────────┐
│   High Nibble (Type)    │    Low Nibble (Flag/Length)           │
├─────────────────────────┼───────────────────────────────────────┤
│ 0 = null                │ 0-13 = length in bytes                │
│ 1 = bool                │ 14   = length follows as VarUInt      │
│ 2 = positive int        │ 15   = null of this type              │
│ 3 = negative int        │                                       │
│ 4 = float               │                                       │
│ 5 = decimal             │                                       │
│ 6 = timestamp           │                                       │
│ 7 = symbol              │                                       │
│ 8 = string              │                                       │
│ 9 = clob                │                                       │
│ A = blob                │                                       │
│ B = list                │                                       │
│ C = sexp                │                                       │
│ D = struct              │                                       │
│ E = annotation          │                                       │
│ F = reserved            │                                       │
└─────────────────────────┴───────────────────────────────────────┘

Example: 0x8A = string (type 8) with 10-byte (A) body
         0xDE = struct (type D) with VarUInt length following
```

---

## 7. Fragment Types for a Simple Book

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    SIMPLE BOOK FRAGMENT TYPES                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  REQUIRED ROOT FRAGMENTS (fid == ftype)                                 │
│  ────────────────────────────────────────                               │
│  $258   Metadata          - Book metadata (title, author, etc.)         │
│  $259   Storyline         - Content sequence                            │
│  $264   position_map      - EID → section mapping                      │
│  $265   position_id_map   - PID → (EID, offset) mapping                │
│  $389   book_navigation   - Navigation per reading order                │
│  $419   container_entity_map - Fragment/container associations          │
│  $538   document_data     - Reading orders, document structure          │
│  $550   location_map      - Location list for pagination                │
│                                                                         │
│  REQUIRED INSTANCE FRAGMENTS (fid != ftype)                             │
│  ────────────────────────────────────────────                           │
│  $157   kfx_style         - Named style definitions                     │
│  $164   external_resource - Resource descriptors (images, etc.)         │
│  $260   section           - Content sections/chapters                   │
│  $266   anchor            - Position/URI anchors for navigation         │
│  $269   text_block        - Text content blocks                         │
│  $417   bcRawMedia        - Raw resource bytes (images)                 │
│                                                                         │
│  COMMON OPTIONAL FRAGMENTS                                              │
│  ────────────────────────────────────────                               │
│  $277   container_block   - Layout container structures                 │
│  $418   bcRawFont         - Raw font data (embedded fonts)              │
│  $490   BookMetadata      - Structured metadata (categorized)           │
│  $585   format_capabilities - Feature flags and capabilities            │
│  $593   fcFCapabilities   - Capability details (when present)           │
│  $597   yj.eid_offset     - EID offset information                      │
│                                                                         │
│  NOT IN SIMPLE BOOKS (omitted)                                          │
│  ────────────────────────────────────────                               │
│  $262, $267, $270*, $387, $390, $391, $393, $394, $418,                 │
│  $608, $609, $610, $611, $621, $692, $756                               │
│                                                                         │
│  * $270 (container) is reconstructed from metadata, not stored          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Fragment Reference Graph (Simple Book)

```
                          ┌────────────────┐
                          │   $538         │
                          │ document_data  │
                          │ (reading_orders│
                          │  $169 list)    │
                          └───────┬────────┘
                                  │ references
                                  ▼
┌─────────────┐           ┌────────────────┐           ┌─────────────┐
│    $258     │──────────▶│     $260       │◀──────────│    $419     │
│  Metadata   │           │   section      │           │ entity_map  │
│ (title,etc) │           │ (chapter/page) │           │($252 list)  │
└─────────────┘           └───────┬────────┘           └─────────────┘
                                  │
                  ┌───────────────┼───────────────┐
                  │               │               │
                  ▼               ▼               ▼
          ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
          │    $259     │ │    $269     │ │    $277     │
          │  Storyline  │ │ text_block  │ │container_blk│
          └─────────────┘ └──────┬──────┘ └──────┬──────┘
                                 │               │
                                 │    ┌──────────┘
                                 │    │
                                 ▼    ▼
                         ┌─────────────────┐
                         │      $157       │
                         │   kfx_style     │
                         │(named styles)   │
                         └─────────────────┘

          ┌─────────────────────────────────────────┐
          │                                         │
          ▼                                         │
  ┌─────────────┐                           ┌─────────────┐
  │    $164     │───────────────────────────│    $266     │
  │  ext_res    │     resource              │   anchor    │
  │(descriptors)│◀──── reference ───────────│ (nav target)│
  └──────┬──────┘                           └──────┬──────┘
         │                                         │
         │ $165 location                           │ $183 position
         ▼                                         │
  ┌─────────────┐                                  │
  │    $417     │                                  │
  │  bcRawMedia │                                  │
  │(image bytes)│                                  │
  └─────────────┘                                  │
                                                   │
  ┌─────────────┐                                  │
  │    $389     │──────────────────────────────────┘
  │book_navig'n │     anchor references
  │($392 list)  │
  └─────────────┘

  Position/Location Mapping:
  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐
  │    $264      │────▶│    $265      │────▶│    $550     │
  │ position_map │     │position_id_  │     │ location_map│
  │(EID→section)│     │map (PID→EID)│     │ (LOC→PID)  │
  └──────────────┘     └──────────────┘     └─────────────┘
```

---

## 9. Sample container_info Decoding

```
Example container_info values for a simple book:

{
  $409: "CR!ABCD1234EFGH5678",     // Container ID
  $410: 0,                         // No compression
  $411: 0,                         // No DRM
  $412: 4096,                      // Chunk size
  $413: 18,                        // Entity dir starts at byte 18
  $414: 432,                       // Entity dir is 432 bytes (18 entries × 24)
  $415: 450,                       // Doc symbols at byte 450
  $416: 128,                       // Doc symbols is 128 bytes
  $594: 578,                       // Format cap at byte 578 (v2 only)
  $595: 64                         // Format cap is 64 bytes
}

Visual byte map (approximate):

0         18        450       578       642       850      1024
│         │         │         │         │         │         │
▼         ▼         ▼         ▼         ▼         ▼         ▼
┌─────────┬─────────┬─────────┬─────────┬─────────┬─────────┬─────────────┐
│ Fixed   │ Entity  │Doc Sym  │Fmt Cap  │container│ kfxgen  │   ENTY      │
│ Header  │Directory│ Table   │($593)   │ _info   │ metadata│   Records   │
│ (18B)   │ (432B)  │ (128B)  │ (64B)   │ Ion     │ JSON    │   ...       │
└─────────┴─────────┴─────────┴─────────┴─────────┴─────────┴─────────────┘
                                                            ▲
                                                            │
                                                      header_len = 1024
```

---

## 10. Complete File Layout Example

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        KFX File Complete Layout                             │
│                          (Simple 3-Chapter Book)                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│ OFFSET    LENGTH   CONTENT                                                  │
│ ──────────────────────────────────────────────────────────────────────────  │
│ 0x0000    18       Fixed Header                                             │
│                    "CONT" + version=2 + header_len + offsets                │
│                                                                             │
│ 0x0012    576      Entity Directory (24 entities × 24 bytes)                │
│                    Entry[0]: $348/$538  document_data                       │
│                    Entry[1]: $348/$258  metadata                            │
│                    Entry[2]: $348/$259  storyline                           │
│                    Entry[3]: ch1/$260   section "chapter_1"                 │
│                    Entry[4]: ch2/$260   section "chapter_2"                 │
│                    Entry[5]: ch3/$260   section "chapter_3"                 │
│                    Entry[6]: tb1/$269   text_block_1                        │
│                    Entry[7]: tb2/$269   text_block_2                        │
│                    ...                                                      │
│                    Entry[20]: img1/$164 resource_descriptor                 │
│                    Entry[21]: img1/$417 raw_media (JPEG bytes)              │
│                    Entry[22]: $348/$264 position_map                        │
│                    Entry[23]: $348/$265 position_id_map                     │
│                                                                             │
│ 0x0252    256      Doc Symbol Table                                         │
│                    $ion_symbol_table annotation                             │
│                    imports: [{name:"$ion", max_id:9},                       │
│                              {name:"YJ_symbols", max_id:806}]               │
│                    symbols: ["chapter_1", "chapter_2", "chapter_3",         │
│                              "style_body", "img_cover", ...]                │
│                                                                             │
│ 0x0352    96       Format Capabilities (v2)                                 │
│                    $593 annotation with feature flags                       │
│                                                                             │
│ 0x03B2    128      container_info (Ion struct)                              │
│                    All $409-$595 fields                                     │
│                                                                             │
│ 0x0432    64       kfxgen Metadata                                          │
│                    [{key: kfxgen_package_version, value: "..."},            │
│                     {key: kfxgen_application_version, value: "..."},        │
│                     {key: kfxgen_payload_sha1, value: "abc123..."},         │
│                     {key: kfxgen_acr, value: "CR!ABCD..."}]                 │
│                                                                             │
│ ═══════════════════════════════════════════════════════════════════════════ │
│ 0x0472    (varies) header_len boundary                                      │
│ ═══════════════════════════════════════════════════════════════════════════ │
│                                                                             │
│ 0x0472    varies   ENTY Record: $538 document_data                          │
│                    "ENTY" + entity_info + Ion struct payload                │
│                                                                             │
│ ...       ...      ENTY Record: $258 metadata                               │
│                    "ENTY" + entity_info + Ion struct payload                │
│                                                                             │
│ ...       ...      ENTY Record: $260 section "chapter_1"                    │
│                    "ENTY" + entity_info + Ion struct payload                │
│                                                                             │
│ ...       ...      ENTY Records for all other fragments...                  │
│                                                                             │
│ ...       large    ENTY Record: $417 raw_media                              │
│                    "ENTY" + entity_info + RAW BYTES (no Ion)                │
│                                                                             │
│ EOF                                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 11. Key Points Summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           KEY POINTS                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. ALL integer fields are LITTLE-ENDIAN                                    │
│                                                                             │
│  2. Entity offsets in directory are RELATIVE to header_len, not file start  │
│                                                                             │
│  3. Ion Binary signature: E0 01 00 EA (always present in Ion streams)       │
│                                                                             │
│  4. Root fragments use id_idnum = $348 (placeholder) in entity directory    │
│                                                                             │
│  5. Raw media ($417) and raw fonts ($418) are NOT Ion-encoded               │
│     - Stored as raw bytes directly                                          │
│                                                                             │
│  6. Symbol IDs ($NNN) are indexes into the combined symbol table:           │
│     - $ion system symbols (1-9)                                             │
│     - YJ_symbols shared catalog                                             │
│     - Local symbols from $ion_symbol_table                                  │
│                                                                             │
│  7. $270 (container fragment) is NOT stored as an ENTY                      │
│     - Reconstructed from container_info and kfxgen metadata                 │
│                                                                             │
│  8. payload_sha1 validates integrity of entire entity payload region        │
│     - SHA1(file[header_len:]) must match kfxgen_payload_sha1                │
│                                                                             │
│  9. Version 2 containers MAY include format capabilities ($593/$594/$595)   │
│                                                                             │
│ 10. ENTY records have their own internal header_len (≥10 bytes minimum)     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 12. Symbol Resolution Flow

```
            Symbol ID Resolution Process
            ═══════════════════════════

  Entity Directory         Symbol Table             Fragment
  Entry contains:          resolves to:             becomes:
  ┌─────────────┐         ┌─────────────┐         ┌─────────────┐
  │ id_idnum=   │────────▶│ Symbol #823 │────────▶│ fid =       │
  │    823      │         │ ="chapter_1"│         │ "chapter_1" │
  │             │         │             │         │             │
  │ type_idnum= │────────▶│ Symbol #260 │────────▶│ ftype =     │
  │    260      │         │ ="$260"     │         │ "$260"      │
  └─────────────┘         └─────────────┘         └─────────────┘

  Symbol Table Structure:
  ┌─────────────────────────────────────────────────────────────┐
  │ Index │  Source           │  Example Symbol                 │
  ├───────┼───────────────────┼─────────────────────────────────┤
  │  1-9  │ $ion system       │ $ion_symbol_table, name, ...    │
  │ 10-   │ YJ_symbols shared │ $258, $260, $269, $417, ...     │
  │ 600+  │ Local symbols     │ chapter_1, style_body, ...      │
  └───────┴───────────────────┴─────────────────────────────────┘
```

---

*Generated for KFXInput project - December 2024*
