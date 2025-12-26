# KFX Implementation (keeping history)

## Overview

Implement KFX (Kindle Format X) output generation from FB2 content. The implementation must:

1. Use `github.com/amazon-ion/ion-go/ion` for Ion serialization
2. Support both serialization (write) and deserialization (read) for debugging
3. Pass validation via `testdata/input.py` (KFXInput plugin)
4. Follow project coding standards (Go idioms, structured logging, etc.)
5. Documentation of format is in `docs/kfxstructure.md` and `docs/symdict.md`
6. We do not want to implement every feature supported by KFX, any magazine-specific ($267, $387, $390), dictionary/notebook specific ($609, $610, $611, $621) could be omitted
7. We probably could use inline anchors via $266 rather than doing ($391, $393, $394)
8. Resulting output must pass `testdata/input.py` without errors and warnings (kindle_audit_metadata warnings could be ignored)

## Progress Tracking

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | ✅ Complete | Core Ion/KFX Infrastructure + Generate skeleton |
| Phase 2 | ✅ Complete | Fragment Model |
| Phase 3 | ✅ Complete | Core Fragment Generators |
| Phase 4 | ✅ Complete | Content Fragment Generators - Storyline, Section, Content, Style |
| Phase 5 | ✅ Complete | Navigation + Resources + Anchors |
| Phase 6 | ✅ Complete | Position/Location mapping + content features: $264, $265, $550, $585 |

**Started:** 2025-12-25

## Current Status

### Phase 6 Achievements

- ✅ Navigation fragment ($389 book_navigation) implemented with hierarchical TOC
- ✅ KFX structure aligned with EPUB (body intro as separate storyline, nested TOC entries)
- ✅ Content chunking implemented (paragraphs as separate entries, auto-split at 8KB)
- ✅ Debug output shows all fields correctly (Version, Format, Generator, LocalSymbols)
- ✅ Debug output is written into the per-book temp WorkDir so it is captured by the report zip
- ✅ External resources ($164) + Raw media ($417) implemented and referenced from content
- ✅ Anchors ($266) implemented and emitted only when referenced by FB2 links (avoid "unreferenced fragments" warnings)
- ✅ Position/location mapping implemented and aligned with reference files:
  - $264 position_map
  - $265 position_id_map as list of {pid,eid} structs with PID progression based on content length
  - $550 location_map
- ✅ $585 content_features added (reflow-* and CanonicalFormat live here in reference KFX, not in $593)
- ✅ $593 format_capabilities kept minimal (kfxgen.textBlock)

### Validation Result

`testdata/input.py` passes with no errors; remaining warning is typically "Unknown generator: kfxgen=fbc/…".

## Implementation Plan

### Phase 1: Core Ion/KFX Infrastructure ✅ COMPLETE

| Task | File | Description |
|------|------|-------------|
| 1.1 | `symbols.go` | Symbol Table - YJ_SYMBOLS constants ($10-$602), LargestKnownSymbol = 851 |
| 1.2 | `ionutil.go` | Ion Wrappers - DecodeIon, IonWriter, IonReader with YJ_symbols support |
| 1.3 | `container.go` | KFX Container Format - CONT file read/write, entity directory parsing |
| 1.4 | `fragment.go` | Fragment Model - Fragment struct, FragmentList, helper types |
| 1.5 | `debug.go` | Debug Output - Container.String(), Container.DumpFragments() |
| 1.6 | `generate.go` | Generate Skeleton - Creates minimal KFX container with basic metadata |

### Phase 2: Fragment Model ✅ COMPLETE

| Task | File | Description |
|------|------|-------------|
| 2.1 | `fragment.go`, `symbols.go` | Fragment Types - ROOT_FRAGMENT_TYPES, SINGLETON_FRAGMENT_TYPES, etc. |
| 2.2 | `values.go` | Fragment Values - Position, Length, Style, Content, Navigation builders |

### Phase 3: Core Fragment Generators ✅ COMPLETE

| Task | File | Description |
|------|------|-------------|
| 3.1 | `frag_container.go` | Container Fragment - $270 with entity list |
| 3.2 | `container.go` | Symbol Table Fragment - buildDocSymbolTable() handles symbol table |
| 3.3 | `frag_entitymap.go` | Container Entity Map - $419 with $252 and $253 dependencies |
| 3.4 | `frag_metadata.go` | Metadata Fragments - $258, $490, $538 |
| 3.5 | `frag_capabilities.go` | Format Capabilities - $593 KFX v2 capabilities |

### Phase 4: Content Fragment Generators ✅ COMPLETE

| Task | File | Description |
|------|------|-------------|
| 4.1 | `frag_storyline.go` | Storyline Fragment - $259 with content_list ($146) |
| 4.2 | `frag_storyline.go` | Section Fragments - $260 with page_templates ($141) |
| 4.3 | `frag_content.go`, `frag_storyline.go` | Content Elements - Text ($269), containers, FB2 processing |
| 4.4 | `frag_style.go` | Style Fragments - $157, StyleRegistry with usage tracking |
| 4.5 | `frag_metadata.go` | Reading Orders - $258 and $538 reading_orders |

### Phase 5: Resource & Navigation Fragments ✅ COMPLETE

| Task | File | Description |
|------|------|-------------|
| 5.1 | `frag_resource.go` | External Resources - $164 with dimensions + format |
| 5.2 | `frag_resource.go` | Raw Media - $417 binary image data |
| 5.3 | `frag_anchor.go` | Anchor Fragments - $266 with position references |
| 5.4 | `frag_storyline.go` | Navigation Fragments - $389 BookNavigation with hierarchical TOC |

### Phase 6: Position Mapping ✅ COMPLETE

| Task | File | Description |
|------|------|-------------|
| 6.1 | `frag_positionmaps.go` | Position Map - $264 EID to section mapping |
| 6.2 | `frag_positionmaps.go` | Position ID Map - $265 PID to EID mapping with sparse progression |
| 6.3 | `frag_positionmaps.go` | Location Map - $550 for Kindle locations |
| 6.4 | `frag_contentfeatures.go` | Content Features - $585 reflow-* and CanonicalFormat |

## Files Created

```
convert/kfx/
├── generate.go          # Main Generate function
├── symbols.go           # Symbol table definitions
├── ionutil.go           # Ion helper utilities
├── container.go         # CONT format read/write
├── fragment.go          # Fragment model
├── values.go            # Value construction helpers
├── debug.go             # Debug output
├── frag_container.go    # $270 Container
├── frag_entitymap.go    # $419 ContainerEntityMap
├── frag_metadata.go     # $258, $490, $538
├── frag_capabilities.go # $593 FormatCapabilities
├── frag_storyline.go    # $259 Storyline, $260 Section, $389 Navigation
├── frag_content.go      # Content elements
├── frag_style.go        # $157 Style
├── frag_resource.go     # $164 ExternalResource, $417 RawMedia
├── frag_anchor.go       # $266 Anchor
├── frag_positionmaps.go # $264/$265/$550 Position/Location maps
├── frag_contentfeatures.go # $585 ContentFeatures
├── frag_positionmaps_test.go # Unit tests
└── images_used.go       # Image usage tracking

cmd/debug/kfxdump/
└── main.go              # KFX dump utility
```

## Symbol Categories Reference

Key symbol ranges from `docs/symdict.md`:

| Range | Category |
|-------|----------|
| $10-$155 | Style/formatting properties |
| $156-$200 | Content/document structure |
| $212-$260 | Navigation, metadata |
| $269-$282 | Content types (text, image, list, table, etc.) |
| $284-$287 | Format types (PNG, JPG, GIF) |
| $306-$395 | Units, conditions, more navigation |
| $409-$419 | Container-specific symbols |
| $538-$611 | Advanced features (document data, position maps) |
| $756-$770 | Ruby/annotation content |

## Notes

### Omitted Features (as requested)

- $267 (SectionMetadata) - magazine specific
- $387 (PreviewImages) - magazine specific
- $390 (SectionNavigation) - magazine specific
- $609 (SectionPositionIDMap) - dictionary/sectionized position maps
- $610 (EIDHashEIDSectionMap) - dictionary/notebook specific
- $611 (SectionPIDCountMap) - dictionary specific
- $621 (LocationPIDMap) - dictionary specific
- Complex navigation via $391/$393/$394 - use inline anchors ($266) instead

### Ion-Go Considerations

- ion-go writes Ion Binary by default with BVM (E0 01 00 EA)
- Need custom symbol table handling for YJ_symbols
