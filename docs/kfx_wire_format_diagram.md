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
├─────────────────────────────────────────────────────────────────────────┤
│ IMPORTANT: Symbol IDs use KFX numbering (see section 12):               │
│ - YJ_symbols: IDs 1-851 (no Ion system symbol offset)                   │
│ - Local symbols: IDs 852+ (e.g., "chapter_1" = 852 if first local)      │
│ - To look up in doc symbol table, add 9 (Ion system symbols offset)     │
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
│  $258   Metadata          - Reading orders (section list references)    │
│  $259   Storyline         - Content sequence                            │
│  $264   position_map      - EID → section mapping                       │
│  $265   position_id_map   - PID → (EID, offset) mapping                 │
│  $389   book_navigation   - Navigation (TOC, page list) per read order  │
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
│  $145   content           - Content fragments (paragraph text pools)    │
│  $262   font              - Embedded font declaration (references $418) │
│  $277   container_block   - Layout container structures                 │
│  $418   bcRawFont         - Raw font data (embedded fonts)              │
│  $490   BookMetadata      - Categorized metadata (title, author, etc.)  │
│  $585   content_features  - Reflow/canonical format features            │
│  $593   format_capabilities - KFX format feature flags (header blob)    │
│  $597   yj.eid_offset     - EID offset information                      │
│                                                                         │
│  NOTE: $258 and $490 serve different purposes:                          │
│  - $258 contains reading_orders ($169) - document structure             │
│  - $490 contains categorised_metadata ($491) - title, author, etc.      │
│  Both are typically present in modern KFX files.                        │
│                                                                         │
│  NOT IN SIMPLE BOOKS (omitted)                                          │
│  ────────────────────────────────────────                               │
│  $267, $270*, $387, $390, $391, $393, $394,                             │
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
│    $258     │──────────>│     $260       │<──────────│    $419     │
│  Metadata   │           │   section      │           │ entity_map  │
│(read orders)│           │ (chapter/page) │           │($252 list)  │
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
  │(descriptors)│<──── reference ───────────│ (nav target)│
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
  │($392 list)  │     Contains nav_containers:
  └─────────────┘     - TOC ($235=$212)
                      - Landmarks ($235=$236) - REQUIRED for cover scaling
                      - APPROXIMATE_PAGE_LIST ($235=$237, $239=local symbol)

  Landmarks Container Structure ($235=$236):
  ┌─────────────────────────────────────────────────────────────────────┐
  │ $247 entries list:                                                  │
  │   Cover:  {$238: $233, $241: {$244: "label"}, $246: {$155: eid}}    │
  │   TOC:    {$238: $212, $241: {$244: "label"}, $246: {$155: eid}}    │
  │   Start:  {$238: $396, $241: {$244: "Start"}, $246: {$155: eid}}    │
  │                                                                     │
  │ NOTE: Cover landmark ($238=$233) is REQUIRED for KP3 to properly    │
  │       scale cover images to fill the screen without white borders.  │
  └─────────────────────────────────────────────────────────────────────┘

  ┌─────────────┐
  │    $490     │  Book Metadata (categorised):
  │ BookMetadata│  - kindle_title_metadata: title, author, ASIN, etc.
  │($491 list)  │  - kindle_audit_metadata: creator info
  └─────────────┘  - kindle_ebook_metadata: capabilities

   Note: kindle_title_metadata includes `cde_content_type`, which this converter sets to `PDOC` or `EBOK`

  Position/Location Mapping:
  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐
  │    $264      │────>│    $265      │────>│    $550     │
  │ position_map │     │position_id_  │     │ location_map│
  │(EID→section) │     │map (PID→EID) │     │ (LOC→PID)   │
  └──────────────┘     └──────────────┘     └─────────────┘

  Content/Format Features:
  ┌───────────────┐     ┌──────────────┐
  │    $585       │     │    $593      │
  │content_feats  │     │format_capabs │
  │(reflow-*, etc)│     │(kfxgen.*)    │
  └───────────────┘     └──────────────┘
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
│ ────────────────────────────────────────────────────────────────────────────│
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
│ ════════════════════════════════════════════════════════════════════════════│
│ 0x0472    (varies) header_len boundary                                      │
│ ════════════════════════════════════════════════════════════════════════════│
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

## 11. Section and Storyline Structure (KP3-Compatible)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    $260 SECTION FRAGMENT STRUCTURE                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Section Fragment (fid = section_name, ftype = $260):                       │
│  {                                                                          │
│    $174: section_name,       // Section ID matching fid                     │
│    $141: [                   // page_templates list                         │
│      {                                                                      │
│        $155: <eid>,          // Page template EID                           │
│        $159: $269,           // Type = text (CRITICAL: not $270!)           │
│        $176: storyline_name  // Reference to $259 fragment                  │
│      }                                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: Page Template Type ($159) must be $269 (text)            │    │
│  │                                                                     │    │
│  │  KP3-Compatible:    { $155: eid, $159: $269, $176: name }           │    │
│  │  Non-Compatible:    { $155: eid, $159: $270, $176: name,            │    │
│  │                       $140: $320, $156: $326, $56: 600, $57: 800 }  │    │
│  │                                                                     │    │
│  │  Using $270 (container) type causes Kindle to show only first       │    │
│  │  page of each section. Always use $269 (text) type.                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    $259 STORYLINE FRAGMENT STRUCTURE                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Storyline Fragment (fid = story_name, ftype = $259):                       │
│  {                                                                          │
│    $176: story_name,         // Storyline ID matching fid                   │
│    $146: [                   // content_list                                │
│      // Text content entry                                                  │
│      {                                                                      │
│        $155: <eid>,          // Element ID                                  │
│        $159: $269,           // Type = text                                 │
│        $157: style_name,     // Optional style reference                    │
│        $790: <level>,        // Heading level 1-6 (KP3 parity, optional)    │
│        $145: {               // Content reference                           │
│          name: content_X,    // Content fragment name                       │
│          $403: <offset>      // Array index within content_list             │
│        },                                                                   │
│        $142: [...]           // Optional style events (inline formatting)   │
│      },                                                                     │
│      // Image content entry                                                 │
│      {                                                                      │
│        $155: <eid>,          // Element ID                                  │
│        $159: $271,           // Type = image                                │
│        $175: resource_name,  // Resource ref as SYMBOL (not string!)        │
│        $584: "alt text"      // Alt text (only if non-empty)                │
│      }                                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  Soft-Hyphen Insertion (§7.10.11):                                          │
│  ─────────────────────────────────                                          │
│  Text strings in $145 content fragments may contain Unicode soft hyphens    │
│  (U+00AD) inserted at conversion time. These are invisible break hints:     │
│  the reading system may hyphenate at those points or ignore them.           │
│  Soft hyphens are inserted for all regular paragraphs when hyphenation is   │
│  enabled; skipped for code/preformatted blocks.                             │
│  This is independent of the $127 (hyphens) style property — $127 controls  │
│  runtime hyphenation behavior, while U+00AD marks specific break points.    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    COVER SECTION STRUCTURE (SPECIAL)                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Cover sections use DIFFERENT structure than regular text sections.         │
│  This enables full-screen cover display without white borders.              │
│                                                                             │
│  Cover Section Fragment ($260, first section "c0"):                         │
│  {                                                                          │
│    $174: "c0",                 // Section name                              │
│    $141: [                     // page_templates list                       │
│      {                                                                      │
│        $140: $320,             // float = center                            │
│        $155: <eid>,            // Page template EID                         │
│        $156: $326,             // layout = scale_fit                        │
│        $159: $270,             // Type = CONTAINER (not $269 text!)         │
│        $176: storyline_name,   // Reference to cover storyline              │
│        $66: <width>,           // Container width (pixels)                  │
│        $67: <height>           // Container height (pixels)                 │
│      }                                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  Cover Storyline ($259):                                                    │
│  {                                                                          │
│    $176: storyline_name,                                                    │
│    $146: [                     // Single image entry                        │
│      {                                                                      │
│        $155: <eid>,            // Element ID                                │
│        $159: $271,             // Type = image                              │
│        $175: resource_name,    // Resource ref as SYMBOL (not string)       │
│        $157: cover_style       // Minimal style: font-size 1rem, lh 1.01lh  │
│        // NOTE: No $584 (alt_text) unless non-empty                         │
│      }                                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: For cover to scale properly (fill screen):               │    │
│  │                                                                     │    │
│  │  1. Section type ($159) must be $270 (container), NOT $269 (text)   │    │
│  │  2. Must include $66/$67 (container dimensions)                     │    │
│  │  3. Must have cover landmark in $389 navigation:                    │    │
│  │     {$238: $233, $241: {$244: "label"}, $246: {$155: cover_eid}}    │    │
│  │                                                                     │    │
│  │  Without the landmark, KP3 does not recognize the section as a      │    │
│  │  cover and renders it with standard margins/white borders.          │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    $142 STYLE EVENTS (INLINE FORMATTING)                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Style Event Entry (element in $142 list):                                  │
│  {                                                                          │
│    $143: <offset>,           // Start offset in text (CHARACTER offset)     │
│    $144: <length>,           // Span length (in CHARACTERS)                 │
│    $157: style_name,         // Style reference (e.g., "emphasis")          │
│    $179: anchor_name,        // Optional link target (see section 13)       │
│    $616: $617                // For footnote links: yj.display = yj.note    │
│  }                                                                          │
│                                                                             │
│  CRITICAL - Character vs Byte Offsets:                                      │
│  ─────────────────────────────────────                                      │
│  Offsets ($143) and lengths ($144) are measured in Unicode code points      │
│  (characters/runes), NOT bytes! This is critical for multi-byte UTF-8:      │
│                                                                             │
│    Text: "Автор 1.1"                                                        │
│    - "Автор" = 5 chars, 10 UTF-8 bytes (Cyrillic)                           │
│    - " " = 1 char, 1 byte                                                   │
│    - "1.1" = 3 chars, 3 bytes                                               │
│                                                                             │
│    Style event for "1.1": $143: 6, $144: 3 (chars, NOT bytes!)              │
│    Wrong (byte offset): $143: 11, $144: 3                                   │
│                                                                             │
│  Link Target ($179):                                                        │
│  ───────────────────                                                        │
│  The $179 field references an anchor fragment ($266) by its anchor_name.    │
│  - For internal links: points to a position anchor                          │
│  - For external links: points to an external URI anchor (see section 13)    │
│                                                                             │
│  Footnote Link Detection (KP3 parity):                                      │
│  ─────────────────────────────────────                                      │
│  Links to footnote bodies should include $616: $617 marker.                 │
│  This enables Kindle's popup footnote display feature.                      │
│                                                                             │
│  CRITICAL - Style Event Ordering and Overlap Rules:                         │
│  ───────────────────────────────────────────────────                        │
│  KP3 enforces strict rules about style event ordering and overlap.          │
│  Violating these causes rendering failures (wrong fonts, broken alignment). │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  OVERLAP RULES (from KP3 source: com.amazon.B.d.e.c.d.java)         │    │
│  │                                                                     │    │
│  │  ALLOWED - Complete nesting (one event fully inside another):       │    │
│  │    [0]: offset=5, len=4   // outer: covers 5-8                      │    │
│  │    [1]: offset=6, len=3   // inner: covers 6-8 (fully inside [0])   │    │
│  │                                                                     │    │
│  │  NOT ALLOWED - Partial overlap (neither fully contains the other):  │    │
│  │    [0]: offset=5, len=4   // covers 5-8                             │    │
│  │    [1]: offset=7, len=4   // covers 7-10 (PARTIAL overlap!)         │    │
│  │    → KP3 throws: "Cannot create Overlapping Style Events"           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Example from KP3 reference (footnote link nested inside superscript):      │
│    [0]: offset=15, len=6, style="s19M"   // superscript: covers 15-20       │
│    [1]: offset=16, len=4, style="s19N"   // link: covers 16-19 (nested!)    │
│  This is VALID because [1] is completely contained within [0].              │
│                                                                             │
│  ORDERING REQUIREMENT:                                                      │
│  ─────────────────────                                                      │
│  Events must be sorted by:                                                  │
│    1. Offset ascending (primary key)                                        │
│    2. Length DESCENDING (secondary key - LONGER events first at same offset)│
│  This ensures outer/containing events come before inner/nested events.      │
│                                                                             │
│  STYLE INHERITANCE FOR NESTED CONTEXTS:                                     │
│  ───────────────────────────────────────                                    │
│  When an inner element (link) appears inside an outer context (superscript),│
│  the inner element's style should INCLUDE properties from the outer context.│
│  Create combined styles that merge outer + inner properties (e.g., s19N     │
│  includes both superscript baseline AND link styling).                      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    $157 STYLE PROPERTY DIMENSIONS                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Dimension Value Structure:                                                 │
│  {                                                                          │
│    $307: <magnitude>,        // MUST be Ion DecimalType (NOT float/string)  │
│    $306: <unit_symbol>       // Unit type symbol                            │
│  }                                                                          │
│                                                                             │
│  CRITICAL - Ion Type for $307:                                              │
│  ─────────────────────────────                                              │
│  The $307 field MUST be encoded as Ion DecimalType. KP3 will crash or       │
│  render incorrectly if $307 is Ion Float, Ion Int, or Ion String.           │
│  Use ion.MustParseDecimal() or equivalent to create proper decimals.        │
│  Decimal notation examples: "2.5d-1" for 0.25, "1." for 1.0, "8d-1" for 0.8 │
│                                                                             │
│  CRITICAL - Decimal Precision (MAX 3 DIGITS):                               │
│  ────────────────────────────────────────────                               │
│  KP3 requires decimal values to have AT MOST 3 significant decimal digits.  │
│  Amazon's code uses setScale(3, RoundingMode.HALF_UP) for dimensions.       │
│  Excessive precision causes RENDERING FAILURES (images don't display).      │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Working (3 digits):    $307: decimal(8.33d-1)        // 0.833      │    │
│  │                         $307: decimal(63.333)         // 63.333     │    │
│  │                                                                     │    │
│  │  BROKEN (too many):     $307: decimal(8.333333333333334d-1)         │    │
│  │                         $307: decimal(63.33333333333333)            │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Unit Symbols:                                                              │
│  ─────────────                                                              │
│  $308 = em        (relative to font size)                                   │
│  $309 = px        (pixels)                                                  │
│  $310 = lh        (line-height units)                                       │
│  $311 = pt        (points)                                                  │
│  $314 = %         (percent)                                                 │
│  $505 = rem       (relative to root font size)                              │
│                                                                             │
│  KP3 Unit Conventions (CRITICAL):                                           │
│  ─────────────────────────────────                                          │
│  Kindle Previewer uses specific units for different properties.             │
│  Using incorrect units can break rendering (e.g., text-align).              │
│                                                                             │
│  ┌────────────────┬──────────┬──────────────────────────────────────────┐   │
│  │ CSS Property   │ KP3 Unit │ Notes                                    │   │
│  ├────────────────┼──────────┼──────────────────────────────────────────┤   │
│  │ font-size      │ rem      │ NOT %. Using % breaks text-align!        │   │
│  │ margin-top     │ lh       │ Line-height units for vertical spacing   │   │
│  │ margin-bottom  │ lh       │ Line-height units for vertical spacing   │   │
│  │ margin-left    │ %        │ Percentage for horizontal spacing        │   │
│  │ margin-right   │ %        │ Percentage for horizontal spacing        │   │
│  │ text-indent    │ %        │ Percentage (1em → 3.125%)                │   │
│  │ line-height    │ lh       │ Line-height units                        │   │
│  │ padding-top    │ lh       │ Line-height units for table cells        │   │
│  │ padding-bottom │ lh       │ Line-height units for table cells        │   │
│  │ padding-left   │ %        │ Percentage for table cells               │   │
│  │ padding-right  │ %        │ Percentage for table cells               │   │
│  └────────────────┴──────────┴──────────────────────────────────────────┘   │
│                                                                             │
│  Unit Conversion from CSS:                                                  │
│  ─────────────────────────────                                              │
│  em → lh (vertical):   1:1 ratio      (1em → 1lh)                           │
│  em → %  (horizontal): 1:6.25 ratio   (1em → 6.25%)                         │
│  em → %  (text-indent): 1:3.125 ratio (1em → 3.125%)                        │
│  %  → rem (font-size): divide by 100  (140% → 1.4rem)                       │
│  em → rem (font-size): 1:1 ratio      (1em → 1rem)                          │
│                                                                             │
│  Ex-to-Em Conversion (§7.10.13):                                            │
│  ────────────────────────────────                                           │
│  KFX does NOT support the CSS "ex" unit. KP3 converts all ex values to em   │
│  early in normalization using: em_value = ex_value × 0.44                   │
│  (factor from com/amazon/yj/F/a/b.java:24).                                 │
│  Examples: 1ex → 0.44em,  2ex → 0.88em,  0.5ex → 0.22em                     │
│  The "ex" unit never appears in KFX output.                                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: Margin-Auto Resolution (§7.10.12)                        │    │
│  │                                                                     │    │
│  │  CSS margin-left/right: auto do NOT become KFX margin properties.   │    │
│  │  KP3's MarginAutoTransformer resolves them:                         │    │
│  │                                                                     │    │
│  │  margin-top/bottom: auto  →  0em (CSS 2.1 §10.6.3)                  │    │
│  │  both margin-left+right auto  →  $587 (box_align) = $320 (center)   │    │
│  │  only margin-left auto  →  $587 = $61 (right)                       │    │
│  │  only margin-right auto  →  $587 = $59 (left)                       │    │
│  │                                                                     │    │
│  │  Auto margins are DELETED from output; existing $587 is preserved.  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Text-Decoration-None Filtering (§7.10.14):                                 │
│  ───────────────────────────────────────────                                │
│  KP3 strips "text-decoration: none" from most elements (it's a no-op).      │
│  Preserved ONLY for: <u>, <a>, <ins>, <del>, <s>, <strike>, <br>.           │
│  For these, it has semantic meaning (e.g., removing inherent underline).    │
│  Note: <a> exemption is reflowable-only; in fixed-layout it is stripped.    │
│                                                                             │
│  Border-Radius Elliptical Values (§7.10.15):                                │
│  ────────────────────────────────────────────                               │
│  CSS border-*-radius with two space-separated values (e.g., "10px 20px")    │
│  encodes as an Ion list of two dimension structs:                           │
│    $97: [ {$307: 10, $306: $309}, {$307: 20, $306: $309} ]                  │
│  If both values are identical → collapsed to single dimension.              │
│  3+ values → rejected by KP3.                                               │
│                                                                             │
│  Line-Height Adjustment (§7.10.16):                                         │
│  ──────────────────────────────────                                         │
│  font-size < 1rem: line-height = 1lh; margins scaled by 1/font-size.        │
│  font-size ≥ 1rem: line-height = 1.0101lh (100/99); margins / line-height.  │
│                                                                             │
│  Zero Value Omission:                                                       │
│  ────────────────────                                                       │
│  KP3 does NOT include style properties with zero values.                    │
│  Example: margin-left: 0 is omitted entirely, NOT encoded as:               │
│    $48: { $307: 0, $306: "$314" }                                           │
│                                                                             │
│  Padding Properties ($52-$55):                                              │
│  ─────────────────────────────                                              │
│  $52 = padding_top     (dimension with lh units)                            │
│  $53 = padding_left    (dimension with % units)                             │
│  $54 = padding_bottom  (dimension with lh units)                            │
│  $55 = padding_right   (dimension with % units)                             │
│  Used primarily for table cell styling.                                     │
│                                                                             │
│  Border Properties ($83, $88, $93):                                         │
│  ─────────────────────────────────                                          │
│  $83 = border_color    (packed ARGB integer, e.g., 0xFF000000 for black)    │
│  $88 = border_style    (symbol: $328=solid, $330=dashed, $331=dotted)       │
│  $93 = border_weight   (dimension with pt units)                            │
│                                                                             │
│  Color Format (ARGB Integer):                                               │
│  ────────────────────────────                                               │
│  Colors stored as packed 32-bit ARGB: 0xAARRGGBB                            │
│  Examples: black=0xFF000000 (4278190080), white=0xFFFFFFFF                  │
│  Applies to: $83 (border_color), $19 (text_color), $70 (fill_color)         │
│                                                                             │
│  Orphans/Widows ($131/$132) - NOT USED:                                     │
│  ───────────────────────────────────────                                    │
│  KP3 does NOT generate $131 (orphans) or $132 (widows) properties.          │
│  Page break control uses $788 (yj_break_after) and $789 (yj_break_before).  │
│                                                                             │
│  Text-Align Symbol Mapping (CRITICAL):                                      │
│  ──────────────────────────────────────                                     │
│  CSS text-align uses PHYSICAL symbols ($59/$61), NOT logical ($680/$681):   │
│    left    → $59  (SymLeft)     right   → $61  (SymRight)                   │
│    center  → $320 (SymCenter)   justify → $321 (SymJustify)                 │
│    start   → $680 (SymStart)    end     → $681 (SymEnd) [rarely used]       │
│  KP3 reference files consistently use $59/$61 for left/right alignment.     │
│                                                                             │
│  Hyphens Symbol Mapping ($127):                                             │
│  ──────────────────────────────                                             │
│  CSS hyphens / -webkit-hyphens → KFX $127 (hyphens):                        │
│    none    → $349 (SymNone)     No hyphenation                              │
│    auto    → $383 (SymAuto)     Automatic hyphenation                       │
│    manual  → $384 (SymManual)   Only at soft hyphen (U+00AD) points         │
│  KFX-internal values $348 (unknown) and $441 (enabled) are not CSS-mapped.  │
│                                                                             │
│  Shape Outside / Border Path ($650):                                        │
│  ───────────────────────────────────                                        │
│  CSS `-amzn-shape-outside: polygon(...)` → KFX $650 (yj.border_path).       │
│  Value is a flat Ion list of float64 encoding a KVG (Kindle Vector          │
│  Graphics) path for float exclusion zones. Commands and coordinates are     │
│  interleaved:                                                               │
│    0 = MOVE_TO    (+ 2 floats: x, y)                                        │
│    1 = LINE_TO    (+ 2 floats: x, y)                                        │
│    4 = CLOSE_PATH (no coordinates)                                          │
│  KP3 only accepts polygon() with percent coordinates, divided by 100        │
│  to produce fractional values (0.0-1.0). First point uses MOVE_TO,          │
│  subsequent points use LINE_TO, path ends with CLOSE_PATH.                  │
│  Example: polygon(0% 0%, 100% 0%, 100% 100%, 0% 100%)                       │
│  →  $650: [0e0, 0e0, 0e0, 1e0, 1e0, 0e0, 1e0, 1e0, 1e0, 1e0, 0e0,           │
│            1e0, 4e0]                                                        │
│                                                                             │
│  Example Style Fragment:                                                    │
│  {                                                                          │
│    $173: "body-title-header",     // Style name                             │
│    $16:  { $307: 1.4, $306: $505 }, // font-size: 1.4rem                    │
│    $34:  $320,                    // text-align: center                     │
│    $36:  { $307: 0, $306: $314 }, // text-indent: 0%                        │
│    $13:  $361,                    // font-weight: bold                      │
│    $47:  { $307: 2, $306: $310 }, // margin-top: 2lh                        │
│    $49:  { $307: 1, $306: $310 }  // margin-bottom: 1lh                     │
│  }                                                                          │
│                                                                             │
│  Example Table Cell Style:                                                  │
│  {                                                                          │
│    $173: "table-cell",                                                      │
│    $83:  4278190080,               // border-color: black (0xFF000000)      │
│    $88:  $328,                     // border-style: solid                   │
│    $93:  { $307: 0.45, $306: $318 }, // border-width: 0.45pt                │
│    $52:  { $307: 0.4, $306: $310 },  // padding-top: 0.4lh                  │
│    $53:  { $307: 1.5, $306: $314 },  // padding-left: 1.5%                  │
│    $54:  { $307: 0.4, $306: $310 },  // padding-bottom: 0.4lh               │
│    $55:  { $307: 1.5, $306: $314 }   // padding-right: 1.5%                 │
│  }                                                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 12. External Resource Structure (`$164`)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    $164 EXTERNAL RESOURCE DESCRIPTOR                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  External Resource Fragment (fid = resource_name, ftype = $164):            │
│  {                                                                          │
│    $175: resource_name,        // MUST be SYMBOL (not string!)              │
│    $161: format_symbol,        // $285=jpg, $284=png, $286=gif              │
│    $162: "image/jpg",          // MIME type (use "image/jpg" not "jpeg")    │
│    $165: "resource/rsrc1",     // Location path (string)                    │
│    $422: <width>,              // Resource width in pixels (optional)       │
│    $423: <height>              // Resource height in pixels (optional)      │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: $175 must be Ion SYMBOL type, not STRING                 │    │
│  │                                                                     │    │
│  │  Correct:   $175: symbol(e1)     // Ion symbol                      │    │
│  │  Wrong:     $175: "e1"           // Ion string - KP3 may fail!      │    │
│  │                                                                     │    │
│  │  CRITICAL: Use "image/jpg" for JPEG files, NOT "image/jpeg"         │    │
│  │  KP3 reference files use "image/jpg" consistently.                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Format Symbols:                                                            │
│  ───────────────                                                            │
│  $285 = jpg (JPEG image)                                                    │
│  $284 = png (PNG image)                                                     │
│  $286 = gif (GIF image)                                                     │
│  $565 = pdf (PDF document)                                                  │
│  $548 = jxr (JPEG XR image)                                                 │
│                                                                             │
│  Example:                                                                   │
│  {                                                                          │
│    $175: symbol(e1),           // Resource name as SYMBOL                   │
│    $161: $285,                 // Format = jpg                              │
│    $162: "image/jpg",          // MIME type                                 │
│    $165: "resource/rsrc1",     // Location                                  │
│    $422: 1264,                 // Width                                     │
│    $423: 1680                  // Height                                    │
│  }                                                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 12.1 Embedded Font Structure (`$262` + `$418`)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    $262 FONT FRAGMENT STRUCTURE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Embedded fonts use two fragment types working together:                    │
│  - $262 (font): Font declaration with metadata and location reference       │
│  - $418 (bcRawFont): Raw font file data (TTF, OTF) as blob                  │
│                                                                             │
│  Font Fragment (fid = font_id, ftype = $262):                               │
│  {                                                                          │
│    $11: "nav-paragraph",     // font_family (with "nav-" prefix)            │
│    $12: $350,                // font_style: $350 (normal), $382 (italic)    │
│    $13: $350,                // font_weight: $350, $361 (bold), $362, etc.  │
│    $15: $350,                // font_stretch: always $350 (normal)          │
│    $165: "resource/rsrc42"   // location: path to bcRawFont fragment        │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  FONT FAMILY NAMING CONVENTION                                      │    │
│  │                                                                     │    │
│  │  KFX uses "nav-" prefix for embedded font family names:             │    │
│  │    CSS: font-family: "paragraph"  →  KFX: $11: "nav-paragraph"      │    │
│  │    CSS: font-family: "dropcaps"   →  KFX: $11: "nav-dropcaps"       │    │
│  │                                                                     │    │
│  │  This distinguishes embedded fonts from system fonts.               │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Font Weight Symbols:                                                       │
│  ────────────────────                                                       │
│  $350 = normal (400)                                                        │
│  $362 = semibold (500-599)                                                  │
│  $361 = bold (700+)                                                         │
│                                                                             │
│  Font Style Symbols:                                                        │
│  ───────────────────                                                        │
│  $350 = normal                                                              │
│  $382 = italic                                                              │
│                                                                             │
│  bcRawFont Fragment ($418):                                                 │
│  ─────────────────────────                                                  │
│  fid = location path (e.g., "resource/rsrc42")                              │
│  ftype = $418                                                               │
│  value = raw font bytes (NOT Ion-encoded, stored as blob)                   │
│                                                                             │
│  Example (font family with normal + bold variants):                         │
│  ──────────────────────────────────────────────────                         │
│                                                                             │
│  // Font declaration for normal weight                                      │
│  $262 (fid="font_0"):                                                       │
│    { $11: "nav-paragraph", $12: $350, $13: $350, $15: $350,                 │
│      $165: "resource/rsrc10" }                                              │
│                                                                             │
│  // Font declaration for bold weight                                        │
│  $262 (fid="font_1"):                                                       │
│    { $11: "nav-paragraph", $12: $350, $13: $361, $15: $350,                 │
│      $165: "resource/rsrc11" }                                              │
│                                                                             │
│  // Raw font data                                                           │
│  $418 (fid="resource/rsrc10"): <raw TTF bytes for normal>                   │
│  $418 (fid="resource/rsrc11"): <raw TTF bytes for bold>                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    BODY FONT AND METADATA FLAG                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  When embedded fonts are used for body text, two additional changes:        │
│                                                                             │
│  1. document_data ($538) includes font_family reference:                    │
│     {                                                                       │
│       $11: "nav-paragraph",        // font_family for body text             │
│       $169: [...]                  // reading_orders                        │
│     }                                                                       │
│                                                                             │
│  2. book_metadata ($490) includes override flag:                            │
│     kindle_title_metadata: [                                                │
│       ...,                                                                  │
│       { $492: "override_kindle_font", $307: "true" }                        │
│     ]                                                                       │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  override_kindle_font FLAG                                          │    │
│  │                                                                     │    │
│  │  When set to "true", tells Kindle to use the embedded font instead  │    │
│  │  of the reader's selected font. Without this flag, embedded fonts   │    │
│  │  may be ignored in favor of user preferences.                       │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 13. Anchor Structure (`$266`)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    $266 ANCHOR FRAGMENT STRUCTURE                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Anchor fragments support two types: position anchors and external URI      │
│  anchors. Both are referenced from style events via $179 (link_to).         │
│                                                                             │
│  POSITION ANCHOR (for internal book links):                                 │
│  ─────────────────────────────────────────                                  │
│  Fragment: fid = anchor_id, ftype = $266                                    │
│  {                                                                          │
│    $183: {                       // Position struct                         │
│      $155: <eid>,                // Target content EID                      │
│      $143: <offset>              // Optional offset within content          │
│    }                                                                        │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  IMPORTANT: $143 Offset in Backlink Anchors                         │    │
│  │                                                                     │    │
│  │  For footnote backlinks (the [<] return link from a footnote back   │    │
│  │  to the body text), $143 MUST be set to the character offset        │    │
│  │  (Unicode code points / runes) of the footnote reference within     │    │
│  │  the target paragraph's text content.                               │    │
│  │                                                                     │    │
│  │  Without $143, the Kindle viewer navigates to offset 0 (start of    │    │
│  │  the paragraph). When the footnote reference [1] appears deep       │    │
│  │  into a long paragraph, this lands the reader pages before the      │    │
│  │  actual reference link.                                             │    │
│  │                                                                     │    │
│  │  Section/chapter anchors targeting paragraph boundaries can omit    │    │
│  │  $143 (offset 0 is correct for them).                               │    │
│  │                                                                     │    │
│  │  Example backlink anchor:                                           │    │
│  │    { $183: { $155: 1159, $143: 152 } }                              │    │
│  │  → navigates to character 152 within the content at EID 1159        │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  EXTERNAL URI ANCHOR (for http/https links):                                │
│  ───────────────────────────────────────────                                │
│  Fragment: fid = anchor_id, ftype = $266                                    │
│  {                                                                          │
│    $180: anchor_name,            // Anchor ID as SYMBOL                     │
│    $186: "http://example.org/..." // External URL string                    │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: External Links Work via Anchor Indirection               │    │
│  │                                                                     │    │
│  │  Correct approach (KP3 reference):                                  │    │
│  │    1. Create $266 anchor with $180 (anchor_name) + $186 (uri)       │    │
│  │    2. Style event references anchor via $179 (link_to)              │    │
│  │                                                                     │    │
│  │  Wrong approach (doesn't work):                                     │    │
│  │    Put $186 directly on style event (external links not clickable)  │    │
│  │                                                                     │    │
│  │  Example:                                                           │    │
│  │  ─────────                                                          │    │
│  │  Anchor fragment (fid="aEXT0"):                                     │    │
│  │    { $180: symbol(aEXT0), $186: "http://www.example.org/" }         │    │
│  │                                                                     │    │
│  │  Style event referencing it:                                        │    │
│  │    { $143: 10, $144: 5, $157: "link-external", $179: symbol(aEXT0) }│    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Link Types and Style Event Structure:                                      │
│  ─────────────────────────────────────                                      │
│  Internal link (footnote):                                                  │
│    { $143: offset, $144: length, $157: "link-footnote",                     │
│      $179: symbol(anchor_id), $616: $617 }                                  │
│                                                                             │
│  External link (URL):                                                       │
│    { $143: offset, $144: length, $157: "link-external",                     │
│      $179: symbol(external_anchor_id) }                                     │
│                                                                             │
│  NOTE: $616: $617 (yj.display: yj.note) is added only for footnote links    │
│  to enable Kindle's popup footnote display feature.                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 14. Position Map Format (KP3-Compatible)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    $264 POSITION_MAP EID LIST FORMAT                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Position Map ($264) value is a list of per-section entries:                │
│  [                                                                          │
│    {                                                                        │
│      $174: section_1,        // Section name                                │
│      $181: [1, 2, 3, 4, 5]   // Flat list of EIDs (KP3 format)              │
│    },                                                                       │
│    {                                                                        │
│      $174: section_2,                                                       │
│      $181: [6, 7, 8, 9]                                                     │
│    }                                                                        │
│  ]                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  EID List Format Comparison:                                        │    │
│  │                                                                     │    │
│  │  KP3-Compatible (flat list):     $181: [1, 2, 3, 4, 5]              │    │
│  │  Legacy (compressed pairs):      $181: [[1, 5]]                     │    │
│  │                                                                     │    │
│  │  The compressed [base, count] format expands to base..base+count-1  │    │
│  │  but flat lists are preferred for KP3 compatibility.                │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 14.0.1 Location Map Format (`$550`)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    $550 LOCATION_MAP STRUCTURE                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Location Map ($550) maps reader "locations" to positions within content.   │
│  Each location represents approximately 40 PIDs (text character positions). │
│                                                                             │
│  Structure:                                                                 │
│  ──────────                                                                 │
│  [                                                                          │
│    {                                                                        │
│      $178: "default",              // Reading order name                    │
│      $182: [                       // List of location entries              │
│        { $155: 1001 },             // Location 0: EID 1001, offset 0        │
│        { $155: 1001, $143: 40 },   // Location 1: EID 1001, offset 40       │
│        { $155: 1002 },             // Location 2: EID 1002, offset 0        │
│        { $155: 1002, $143: 15 },   // Location 3: EID 1002, offset 15       │
│        ...                                                                  │
│      ]                                                                      │
│    }                                                                        │
│  ]                                                                          │
│                                                                             │
│  Location Entry Fields:                                                     │
│  ──────────────────────                                                     │
│  $155 (unique_id)  : EID containing this location (required)                │
│  $143 (offset)     : Character offset within the EID (optional, default 0)  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: The $143 offset field                                    │    │
│  │                                                                     │    │
│  │  When a location falls in the MIDDLE of an EID (not at the start),  │    │
│  │  the $143 offset field specifies the character position within      │    │
│  │  that EID where the location begins.                                │    │
│  │                                                                     │    │
│  │  If offset is 0, the $143 field is OMITTED (not set to 0).          │    │
│  │                                                                     │    │
│  │  Example: For text "Hello World" (11 chars) starting at EID 1001:   │    │
│  │    PID  0-10  → Location 0: { $155: 1001 }         (offset 0)       │    │
│  │    PID 40-50  → Location 1: { $155: 1001, $143: 40 } (offset 40)    │    │
│  │                                                                     │    │
│  │  Without $143, all locations would point to EID boundaries only,    │    │
│  │  resulting in fewer locations than KP3 generates.                   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Positions-per-Location Constant:                                           │
│  ────────────────────────────────                                           │
│  LOC = floor(PID / 40)                                                      │
│                                                                             │
│  NOTE: kfxlib uses 110 positions per location, but KP3-generated KFX        │
│  files use 40. This converter uses 40 for KP3 compatibility.                │
│                                                                             │
│  Algorithm (pseudocode):                                                    │
│  ───────────────────────                                                    │
│    pid = 0                                                                  │
│    nextLocationPID = 0                                                      │
│    for each content item with (EID, length):                                │
│      itemStart = pid                                                        │
│      itemEnd = pid + length                                                 │
│      while nextLocationPID < itemEnd:                                       │
│        offset = max(0, nextLocationPID - itemStart)                         │
│        if offset == 0:                                                      │
│          emit { $155: EID }                                                 │
│        else:                                                                │
│          emit { $155: EID, $143: offset }                                   │
│        nextLocationPID += 40                                                │
│      pid = itemEnd                                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 14.1 Inline Images in Mixed Content (Position Tracking)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    INLINE IMAGES IN STORYLINE CONTENT                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  When a paragraph contains inline images (text with embedded images),       │
│  the storyline entry uses a nested content_list ($146) instead of a         │
│  content reference ($145).                                                  │
│                                                                             │
│  Mixed Content Entry Structure:                                             │
│  ─────────────────────────────                                              │
│  {                                                                          │
│    $155: <parent_eid>,         // Parent paragraph EID                      │
│    $159: $269,                 // Type = text                               │
│    $157: style_name,           // Optional style                            │
│    $146: [                     // Mixed content_list (NOT $145!)            │
│      "Text before image ",     // String segment                            │
│      {                         // Inline image struct                       │
│        $155: <image_eid>,      // Image element EID                         │
│        $159: $271,             // Type = image                              │
│        $175: resource_name     // Resource ref as SYMBOL                    │
│      },                                                                     │
│      " text after image ",     // String segment                            │
│      {                         // Another inline image                      │
│        $155: <image_eid_2>,                                                 │
│        $159: $271,                                                          │
│        $175: resource_name_2                                                │
│      },                                                                     │
│      " more text"              // Final string segment                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  KEY DIFFERENCE:                                                    │    │
│  │                                                                     │    │
│  │  Regular text:  $145: {name: content_X, $403: offset}               │    │
│  │  Mixed content: $146: ["text", {image}, "text", ...]                │    │
│  │                                                                     │    │
│  │  The presence of $146 with string+struct items indicates mixed      │    │
│  │  content requiring granular position tracking.                      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    $265 POSITION_ID_MAP FOR INLINE IMAGES                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  For paragraphs with inline images, KP3 generates granular position         │
│  entries that track each image's exact character offset within the text.    │
│                                                                             │
│  Entry Types:                                                               │
│  ────────────                                                               │
│  1. Parent entry:      { $184: pid, $185: parent_eid }                      │
│  2. Before image:      { $143: offset, $184: pid, $185: parent_eid }        │
│  3. Image entry:       { $184: pid, $185: image_eid }                       │
│  4. After image:       { $143: offset+1, $184: pid+1, $185: parent_eid }    │
│                                                                             │
│  Symbol Reference:                                                          │
│  ─────────────────                                                          │
│  $143 = offset (character position within parent text)                      │
│  $184 = position_id (PID - global position counter)                         │
│  $185 = element_id (EID - content element identifier)                       │
│                                                                             │
│  Offset Calculation Rules:                                                  │
│  ─────────────────────────                                                  │
│  - Text characters: +1 per Unicode code point (rune)                        │
│  - Inline images: +1 per image (images consume position space!)             │
│  - "Before image" offset = cumulative text length before image              │
│  - "After image" offset = "before image" offset + 1                         │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: Images Consume Position Space                            │    │
│  │                                                                     │    │
│  │  When calculating offsets for subsequent images, each preceding     │    │
│  │  image adds 1 to the offset counter:                                │    │
│  │                                                                     │    │
│  │  Text: "AAAA" [img1] "BBBBBBBBBB" [img2] "CCCC"                     │    │
│  │         ↑4    ↑+1    ↑10         ↑+1                                │    │
│  │                                                                     │    │
│  │  1st image offset: 4 (after "AAAA")                                 │    │
│  │  2nd image offset: 4 + 1 + 10 = 15 (not 14!)                        │    │
│  │                                                                     │    │
│  │  Wrong:   offset = sum(text_lengths)                                │    │
│  │  Correct: offset = sum(text_lengths) + count(preceding_images)      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Example Position Entries (KP3 Reference):                                  │
│  ─────────────────────────────────────────                                  │
│  Text: "Тэг [img1] может быть вложен [img2] и [img3]..."                    │
│                                                                             │
│  Entry                        │ $143  │ $184  │ $185  │ Notes               │
│  ─────────────────────────────┼───────┼───────┼───────┼─────────────────────│
│  Parent at start              │   -   │ 11609 │  879  │ Paragraph start     │
│  Before 1st image             │   4   │ 11613 │  879  │ After "Тэг "        │
│  1st inline image             │   -   │ 11613 │ 1550  │ Same PID as before  │
│  After 1st image              │   5   │ 11614 │  879  │ offset+1, pid+1     │
│  Before 2nd image             │  31   │ 11640 │  879  │ Text + img1 offset  │
│  2nd inline image             │   -   │ 11640 │ 1264  │                     │
│  After 2nd image              │  32   │ 11641 │  879  │                     │
│  Before 3rd image             │  35   │ 11644 │  879  │ Text + img1 + img2  │
│  3rd inline image             │   -   │ 11644 │ 1551  │                     │
│  After 3rd image              │  36   │ 11645 │  879  │                     │
│                                                                             │
│  PID Calculation:                                                           │
│  ────────────────                                                           │
│  For each entry: pid = start_pid + offset                                   │
│  - Before image:  pid = start_pid + image_offset                            │
│  - Image:         pid = start_pid + image_offset (same as before)           │
│  - After image:   pid = start_pid + image_offset + 1                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    FORMAT_CAPABILITIES FOR OFFSET ENTRIES                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  When $265 position_id_map contains entries with $143 (offset field),       │
│  the format_capabilities ($593) MUST include:                               │
│                                                                             │
│  { $492: "kfxgen.pidMapWithOffset", version: 1 }                            │
│                                                                             │
│  Format Capabilities List Example:                                          │
│  ─────────────────────────────────                                          │
│  [                                                                          │
│    { $492: "kfxgen.textBlock", version: 1 },                                │
│    { $492: "kfxgen.pidMapWithOffset", version: 1 }  // Required for offsets │
│  ]                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  VALIDATION: KFXInput checks this capability                        │    │
│  │                                                                     │    │
│  │  If offset entries exist but capability is missing:                 │    │
│  │    ERROR: FC kfxgen.pidMapWithOffset=None with eid offset present   │    │
│  │                                                                     │    │
│  │  Detection logic:                                                   │    │
│  │    has_offset = any($265 entry has $143 field)                      │    │
│  │    capability = format_capabilities["kfxgen.pidMapWithOffset"] == 1 │    │
│  │    if has_offset != capability: ERROR                               │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    IMAGE-ONLY TEXT ENTRIES (SPECIAL CASE)                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  When a text entry contains ONLY inline images (no actual text), KP3 uses   │
│  simplified position tracking. This commonly occurs with title paragraphs   │
│  that display only an image.                                                │
│                                                                             │
│  Image-Only Entry Structure:                                                │
│  ───────────────────────────                                                │
│  {                                                                          │
│    $155: <wrapper_eid>,        // Parent paragraph EID                      │
│    $159: $269,                 // Type = text                               │
│    $146: [                     // content_list with ONLY image(s)           │
│      {                         // No string elements!                       │
│        $155: <image_eid>,                                                   │
│        $159: $271,                                                          │
│        $175: resource_name                                                  │
│      }                                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  Detection Criteria:                                                        │
│  ───────────────────                                                        │
│  - Entry type is $269 (text)                                                │
│  - $146 contains ONLY image struct(s), no strings                           │
│  - All images have offset 0 (no preceding text)                             │
│  - Total length == number of images                                         │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL DIFFERENCE: Image-Only vs Mixed Content                   │    │
│  │                                                                     │    │
│  │  Image-Only (no text):                                              │    │
│  │    { $184: pid, $185: wrapper_eid }    // Wrapper                   │    │
│  │    { $184: pid, $185: image_eid }      // Image (SAME PID!)         │    │
│  │    → PID advances by 1                                              │    │
│  │    → NO offset ($143) entries                                       │    │
│  │                                                                     │    │
│  │  Mixed Content (text + images):                                     │    │
│  │    { $184: pid, $185: wrapper_eid }              // Wrapper         │    │
│  │    { $143: 5, $184: pid+5, $185: wrapper_eid }   // Before image    │    │
│  │    { $184: pid+5, $185: image_eid }              // Image           │    │
│  │    { $143: 6, $184: pid+6, $185: wrapper_eid }   // After image     │    │
│  │    → PID advances by total text length                              │    │
│  │    → HAS offset ($143) entries                                      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  Position ID Map Comparison:                                                │
│  ───────────────────────────                                                │
│                                                                             │
│  IMAGE-ONLY (wrapper EID 1305, image EID 1306):                             │
│  │ Entry Type    │ $143  │ $184  │ $185 │ Notes                             │
│  ├───────────────┼───────┼───────┼──────┼───────────────────────────────────│
│  │ Wrapper       │   -   │ 11391 │ 1305 │ Text entry                        │
│  │ Image         │   -   │ 11391 │ 1306 │ SAME PID as wrapper               │
│  │ (next entry)  │   -   │ 11392 │ ...  │ PID advanced by 1                 │
│                                                                             │
│  MIXED CONTENT "Text [img] more" (wrapper EID 1305, image EID 1306):        │
│  │ Entry Type    │ $143  │ $184  │ $185 │ Notes                             │
│  ├───────────────┼───────┼───────┼──────┼───────────────────────────────────│
│  │ Wrapper       │   -   │ 11391 │ 1305 │ Text entry start                  │
│  │ Before image  │   5   │ 11396 │ 1305 │ After "Text " (5 chars)           │
│  │ Image         │   -   │ 11396 │ 1306 │ Same PID as before                │
│  │ After image   │   6   │ 11397 │ 1305 │ offset+1, pid+1                   │
│  │ (next entry)  │   -   │ 11401 │ ...  │ PID = start + total length        │
│                                                                             │
│  Wrong (generates validation errors):                                       │
│  ─────────────────────────────────────                                      │
│  Treating image-only as mixed content produces incorrect entries:           │
│  │ Wrapper       │   -   │ 11391 │ 1305 │                                   │
│  │ Before image  │   0   │ 11391 │ 1305 │ WRONG: offset entry not needed    │
│  │ Image         │   -   │ 11391 │ 1306 │                                   │
│  │ After image   │   1   │ 11392 │ 1305 │ WRONG: after entry not needed     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    $264 POSITION_MAP FOR INLINE IMAGES                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  The $264 position_map must include BOTH parent EIDs AND inline image       │
│  EIDs in the section's EID list ($181):                                     │
│                                                                             │
│  {                                                                          │
│    $174: section_name,                                                      │
│    $181: [                                                                  │
│      ...,                                                                   │
│      <parent_eid>,     // Paragraph containing inline images                │
│      <image_eid_1>,    // First inline image EID                            │
│      <image_eid_2>,    // Second inline image EID                           │
│      ...                                                                    │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  EID Order: Parent EID first, then inline image EIDs in document order.     │
│  This ensures proper position resolution during EPUB conversion.            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 14.2 Table Structure in Storyline Content

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    TABLE STRUCTURE IN $259 STORYLINE                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Tables use a nested container structure with dedicated type symbols:       │
│    $278 = table          (outermost table wrapper)                          │
│    $454 = table_body     (body section containing rows)                     │
│    $279 = table_row      (row containing cells)                             │
│    $270 = container      (cell wrapper with border/padding style)           │
│    $269 = text           (cell text content)                                │
│    $271 = image          (cell image content)                               │
│                                                                             │
│  Complete Table Structure:                                                  │
│  ─────────────────────────                                                  │
│  {                                                                          │
│    $155: <table_eid>,              // Table element ID                      │
│    $159: $278,                     // Type = table                          │
│    $157: table_style,              // Table style reference                 │
│    $150: false,                    // table_border_collapse                 │
│    $456: {$307: 0, $306: $310},    // border_spacing_vertical (lh)          │
│    $457: {$307: 0, $306: $314},    // border_spacing_horizontal (%)         │
│    $629: [$581, $326],             // yj.table_features: [pan_zoom, scale]  │
│    $630: $632,                     // yj.table_selection_mode: yj.regional  │
│    $146: [                         // content_list: single body entry       │
│      {                                                                      │
│        $155: <body_eid>,           // Body element ID                       │
│        $159: $454,                 // Type = table_body                     │
│        $146: [                     // content_list: row entries             │
│          { ...row 1... },                                                   │
│          { ...row 2... },                                                   │
│        ]                                                                    │
│      }                                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  Row Structure ($279):                                                      │
│  ─────────────────────                                                      │
│  {                                                                          │
│    $155: <row_eid>,                // Row element ID                        │
│    $159: $279,                     // Type = table_row                      │
│    $146: [                         // content_list: cell entries            │
│      { ...cell 1... },                                                      │
│      { ...cell 2... },                                                      │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  Cell Container Structure ($270):                                           │
│  ─────────────────────────────────                                          │
│  {                                                                          │
│    $155: <cell_eid>,               // Cell container element ID             │
│    $159: $270,                     // Type = container                      │
│    $156: $323,                     // layout = vertical                     │
│    $157: cell_container_style,     // Style with border/padding/valign      │
│    $148: 2,                        // table_column_span (optional)          │
│    $149: 2,                        // table_row_span (optional)             │
│    $146: [                         // content_list: cell content            │
│      { ...text or image entry... }                                          │
│    ]                                                                        │
│  }                                                                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  CRITICAL: Table cells are TRUE containers ($270)                   │    │
│  │                                                                     │    │
│  │  Unlike regular text sections (which use $269 for page templates),  │    │
│  │  table cells MUST use container type ($270) with layout: vertical.  │    │
│  │  Cell content (text/images) is nested in the container's $146.      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    TABLE CELL CONTENT TYPES                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Table cells can contain three types of content:                            │
│                                                                             │
│  1. TEXT-ONLY CELL (uses $145 content reference):                           │
│  ─────────────────────────────────────────────────                          │
│  {                                                                          │
│    $155: <text_eid>,                                                        │
│    $159: $269,                     // Type = text                           │
│    $157: text_style,               // Style with text-align                 │
│    $145: {                         // Content reference                     │
│      name: content_X,                                                       │
│      $403: <offset>                // Index in content fragment             │
│    },                                                                       │
│    $142: [...]                     // Optional style events for formatting  │
│  }                                                                          │
│                                                                             │
│  2. IMAGE-ONLY CELL (direct image entries):                                 │
│  ──────────────────────────────────────────                                 │
│  Cell content_list contains image entry directly (no text wrapper):         │
│  {                                                                          │
│    $155: <image_eid>,                                                       │
│    $159: $271,                     // Type = image                          │
│    $157: image_style,              // Style with box-align for alignment    │
│    $175: resource_name,            // Resource ref as SYMBOL                │
│    $584: "alt text"                // Optional alt text                     │
│  }                                                                          │
│                                                                             │
│  3. MIXED CONTENT CELL (text + inline images, uses $146 content_list):      │
│  ──────────────────────────────────────────────────────────────────────     │
│  {                                                                          │
│    $155: <text_eid>,                                                        │
│    $159: $269,                     // Type = text                           │
│    $157: text_style,               // Style with text-align                 │
│    $146: [                         // Mixed content_list (NOT $145!)        │
│      "Text before ",               // String segment                        │
│      {                             // Inline image                          │
│        $155: <image_eid>,                                                   │
│        $159: $271,                 // Type = image                          │
│        $175: resource_name,        // Resource ref as SYMBOL                │
│        $601: $283                  // render: inline                        │
│      },                                                                     │
│      " text after"                 // String segment                        │
│    ],                                                                       │
│    $142: [...]                     // Style events (offsets relative to     │
│  }                                 //   concatenated text, not images)      │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  DETECTION LOGIC for cell content type:                             │    │
│  │                                                                     │    │
│  │  hasText   = cell contains any non-whitespace text segments         │    │
│  │  hasImages = cell contains any inline image segments                │    │
│  │                                                                     │    │
│  │  if hasImages && !hasText:  → IMAGE-ONLY (direct image entries)     │    │
│  │  if hasImages && hasText:   → MIXED CONTENT (content_list format)   │    │
│  │  if !hasImages:             → TEXT-ONLY (content reference)         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  MIXED CONTENT: Style Events and Offsets                            │    │
│  │                                                                     │    │
│  │  Style event offsets ($143) in mixed content are calculated the     │    │
│  │  same way as in paragraphs:                                         │    │
│  │  - Offsets are relative to CONCATENATED TEXT only                   │    │
│  │  - Inline images do NOT consume offset positions for style events   │    │
│  │  - But images DO consume position space in $265 position_id_map     │    │
│  │                                                                     │    │
│  │  Example: "<b>Bold</b> [img] <i>italic</i>"                         │    │
│  │    Style events:                                                    │    │
│  │      { $143: 0, $144: 4, $157: "strong" }      // "Bold"            │    │
│  │      { $143: 5, $144: 6, $157: "emphasis" }    // "italic"          │    │
│  │    Note: offset 5 is right after "Bold " (5 chars), image ignored   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  IMPORTANT: Spanning styles NOT promoted in table cells             │    │
│  │                                                                     │    │
│  │  Unlike paragraphs where a style spanning 100% of content gets      │    │
│  │  promoted to the block style, table cells ALWAYS use style_events   │    │
│  │  even when the style covers the entire cell content.                │    │
│  │                                                                     │    │
│  │  Cell: <td><strong>Bold text</strong></td>                          │    │
│  │  KP3 generates:                                                     │    │
│  │    { $157: "td-text", $142: [{$143:0, $144:9, $157:"strong"}] }     │    │
│  │  NOT:                                                               │    │
│  │    { $157: "td-text-strong" }  // Wrong - KP3 doesn't do this       │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    TABLE CELL STYLING                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Table cells use two separate styles:                                       │
│                                                                             │
│  1. CONTAINER STYLE (on $270 cell container):                               │
│     - border-color ($83): ARGB integer (e.g., 0xFF000000 for black)         │
│     - border-style ($88): symbol ($328=solid, $330=dashed, etc.)            │
│     - border-width ($93): dimension with pt units                           │
│     - padding-top ($52), padding-bottom ($54): lh units                     │
│     - padding-left ($53), padding-right ($55): % units                      │
│     - yj.vertical-align ($586): $320 (center), $324 (top), $325 (bottom)    │
│                                                                             │
│  2. TEXT/IMAGE STYLE (on content inside cell):                              │
│     For text ($269):                                                        │
│     - text-align ($34): $59 (left), $320 (center), $61 (right), $321 (just) │
│     - line-height ($39): typically 1lh                                      │
│                                                                             │
│     For images ($271):                                                      │
│     - box-align ($587): $59 (left), $320 (center), $61 (right)              │
│                                                                             │
│  Header vs Data Cell Styles:                                                │
│  ────────────────────────────                                               │
│  - th-container / td-container: Border, padding, vertical-align             │
│  - th-text / td-text: Text alignment (th defaults center, td left)          │
│  - th-image / td-image: Image alignment (th defaults center, td left)       │
│                                                                             │
│  Example Container Style (th-container):                                    │
│  {                                                                          │
│    $173: "th-container",                                                    │
│    $83:  4278190080,               // border-color: black                   │
│    $88:  $328,                     // border-style: solid                   │
│    $93:  { $307: 0.45, $306: $318 }, // border-width: 0.45pt                │
│    $52:  { $307: 0.417, $306: $310 }, // padding-top: 0.417lh               │
│    $53:  { $307: 1.563, $306: $314 }, // padding-left: 1.563%               │
│    $54:  { $307: 0.417, $306: $310 }, // padding-bottom: 0.417lh            │
│    $55:  { $307: 1.563, $306: $314 }, // padding-right: 1.563%              │
│    $586: $320                      // yj.vertical-align: center             │
│  }                                                                          │
│                                                                             │
│  Example Text Style (th-text):                                              │
│  {                                                                          │
│    $173: "th-text",                                                         │
│    $34:  $320,                     // text-align: center                    │
│    $39:  { $307: 1, $306: $310 }   // line-height: 1lh                      │
│  }                                                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                    TABLE POSITION MAP HANDLING                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Table structures ($278, $454, $279) do NOT appear in $264/$265 maps.       │
│  Only the cell content EIDs (text $269, image $271) are tracked.            │
│                                                                             │
│  Empty Cells Special Case:                                                  │
│  ─────────────────────────                                                  │
│  When a table cell has no content (empty $146 content_list), the cell       │
│  container ($270) MUST still appear in $264 position_map but NOT in         │
│  $265 position_id_map. This prevents validation errors.                     │
│                                                                             │
│  Detection: Entry has $159=$270 (container) AND $156 (layout) field,        │
│  but empty or missing $146 content_list.                                    │
│                                                                             │
│  $264 (position_map):                                                       │
│    $181: [..., <empty_cell_eid>, ...]   // Include empty cell container     │
│                                                                             │
│  $265 (position_id_map):                                                    │
│    // Empty cell OMITTED - no PID entry generated                           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 15. Key Points Summary

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
│  6. Symbol IDs use KFX numbering (NOT standard Ion numbering):              │
│     - IDs 1-851: YJ_symbols shared catalog ($10 to $860)                    │
│     - IDs 852+: Local symbols (book-specific names)                         │
│     - Ion system symbols (1-9) are NOT counted in KFX ID space              │
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
│ 11. CRITICAL: KFX symbol IDs vs Standard Ion IDs have a 9-ID offset:        │
│     - KFX local symbol 0: ID 852 (kfxlib/Kindle expectation)                │
│     - Standard Ion local symbol 0: ID 861 (after Ion system + YJ_symbols)   │
│     - Entity directory and symbol values use KFX numbering                  │
│                                                                             │
│ 12. CRITICAL: Style dimension units must match KP3 conventions:             │
│     - font-size: use rem (NOT %). Percent breaks text-align rendering       │
│     - margin-top/bottom: use lh (line-height units)                         │
│     - margin-left/right: use % (percent)                                    │
│     - text-indent: use % (1em = 3.125%)                                     │
│     - Zero values should be omitted entirely from style properties          │
│                                                                             │
│ 13. CRITICAL: Color values use packed ARGB integers (0xAARRGGBB):           │
│     - black = 0xFF000000 (4278190080)                                       │
│     - white = 0xFFFFFFFF (4294967295)                                       │
│     - Applies to $83 (border_color), $19 (text_color), $70 (fill_color)     │
│                                                                             │
│ 14. CRITICAL: KP3 does NOT use orphans ($131) or widows ($132):             │
│     - Use $788 (yj_break_after) and $789 (yj_break_before) instead          │
│     - If CSS converter generates $131/$132, convert and delete them         │
│                                                                             │
│ 15. Table styling requires padding ($52-$55) and border ($83, $88, $93):    │
│     - Padding: $52 top, $53 left, $54 bottom, $55 right                     │
│     - Border: $83 color (ARGB int), $88 style (symbol), $93 weight (dim)    │
│                                                                             │
│ 16. CRITICAL: Inline images in mixed content require special handling:      │
│     - Use $146 content_list (not $145) with interleaved strings + images    │
│     - Each inline image consumes 1 position in the offset counter           │
│     - $265 must include granular before/image/after entries with $143       │
│     - $264 must include both parent and inline image EIDs                   │
│     - format_capabilities must include kfxgen.pidMapWithOffset=1            │
│                                                                             │
│ 17. CRITICAL: Image-only text entries (no text, only inline images):        │
│     - Detected when $146 has only image struct(s), no strings               │
│     - Use SIMPLIFIED position entries: wrapper + image at SAME PID          │
│     - NO offset ($143) entries - do NOT use before/after pattern            │
│     - PID advances by 1 (not by text length)                                │
│     - Common in title paragraphs with decorative images                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 15. Symbol Resolution Flow

```
            Symbol ID Resolution Process (KFX Numbering)
            ═════════════════════════════════════════════

  Entity Directory         Symbol Table             Fragment
  Entry contains:          resolves to:             becomes:
  ┌─────────────┐         ┌─────────────┐         ┌─────────────┐
  │ id_idnum=   │────────>│ Symbol #852 │────────>│ fid =       │
  │    852      │         │ ="chapter_1"│         │ "chapter_1" │
  │             │         │ (local #0)  │         │             │
  │ type_idnum= │────────>│ Symbol #260 │────────>│ ftype =     │
  │    260      │         │ ="section"  │         │ "$260"      │
  └─────────────┘         └─────────────┘         └─────────────┘

  KFX Symbol ID Numbering (differs from standard Ion):
  ┌───────────────────────────────────────────────────────────────────────────┐
  │ KFX ID   │ Standard Ion ID │  Source           │  Example Symbol          │
  ├──────────┼─────────────────┼───────────────────┼──────────────────────────┤
  │   N/A    │      1-9        │ $ion system       │ $ion_symbol_table, ...   │
  │  1-851   │     10-860      │ YJ_symbols shared │ $258, $260, $269, ...    │
  │  852+    │     861+        │ Local symbols     │ chapter_1, style_body    │
  └──────────┴─────────────────┴───────────────────┴──────────────────────────┘

  IMPORTANT: KFX readers (kfxlib, sync2kindle, Kindle) expect symbol IDs
  without the Ion system symbol offset. Entity directory id_idnum/type_idnum
  and symbol values in payloads use KFX numbering (852+ for local symbols).

  When using standard Ion libraries (like Go's ion-go), you must:
  1. Add 9 to entity directory IDs to look up in doc symbol table
  2. Manually resolve symbol IDs 852+ using the local symbols list
  3. Adjust doc symbol table max_id by ±9 when reading/writing
```

---
