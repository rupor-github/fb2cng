# KFX Implementation TODO

## Overview

Implement KFX (Kindle Format X) output generation from FB2 content. The implementation must:
1. Use `github.com/amazon-ion/ion-go/ion` for Ion serialization
2. Support both serialization (write) and deserialization (read) for debugging
3. Pass validation via `testdata/input.py` (KFXInput plugin)
4. Follow project coding standards (Go idioms, structured logging, etc.)
5. Documentation of format are in docs/kfxsturcrure.md and docs/symdict.md
6. We do not want to implement every feature supported by KFX, any magazine-specific ($267, $387, $390), dictionary/notebook specific ($609, $610, $611, $621) could be omitted.
7. We probably could use inline anchors via $266 rather than doing ($391, $393, $394).
8. Resulting output must pass testdata/input.py without errors and warnings (kindle_audit_metadata warnings could be ignored).
9. Examples of proper working kfx files could be found in /mnt/d/test - our results should resemble them.

## Progress Tracking

- **Started:** 2025-12-25
- **Phase 1 Complete:** ✅ (Core Ion/KFX Infrastructure + Generate skeleton)
- **Phase 2 Complete:** ✅ (Fragment Model)
- Phase 3 Complete: [ ]
- Phase 4 Complete: [ ]
- Phase 5 Complete: [ ]
- Phase 6 Complete: [ ]
- Phase 7 Complete: [ ]
- Phase 8 Complete: [ ]

## Implementation Plan

### Phase 1: Core Ion/KFX Infrastructure (`convert/kfx/`) ✅ COMPLETE

- [x] **1.1 Symbol Table** (`symbols.go`)
  - Define YJ_SYMBOLS constants (key symbol IDs from $10-$602)
  - `LargestKnownSymbol = 851` (as of Kindle Previewer 3.101.0)
  - Helper functions: `SymbolID(name)`, `SymbolName(id)`, `FormatSymbol()`
  - Fragment type maps: `RAW_FRAGMENT_TYPES`, `ROOT_FRAGMENT_TYPES`, `CONTAINER_FRAGMENT_TYPES`

- [x] **1.2 Ion Wrappers** (`ionutil.go`)
  - Shared symbol table with placeholder names (`$10`, `$11`, etc.) up to `LargestKnownSymbol`
  - `DecodeIon(prolog, data, v)` - decode Ion binary using struct tags like `ion:"$409"`
  - `DecodeSymbolTable(data)` - decode Ion symbol table for local symbols
  - `IonWriter` - write Ion binary with YJ_symbols
  - `IonReader` - read Ion binary values into generic `map[string]any`
  - Support all Ion types: bool, int, float, decimal, timestamp, string, symbol, blob, list, struct

- [x] **1.3 KFX Container Format** (`container.go`)
  - Structs with Ion tags: `containerHeader`, `containerInfo`, `entityHeader`, `entityInfo`
  - `ReadContainer(data)` - parse CONT file, decode entities into fragments
  - `WriteContainer()` - serialize fragments back to CONT format
  - Entity directory parsing (24-byte entries)
  - Raw fragment handling ($417, $418)

- [x] **1.4 Fragment Model** (`fragment.go`)
  - `Fragment` struct with FType, FID, Value
  - `FragmentList` with indexing by type and key
  - Helper types: `StructValue`, `ListValue`, `SymbolValue`, `RawValue`
  - Builder methods: `NewStruct()`, `Set()`, `SetString()`

- [x] **1.5 Debug Output** (`debug.go`)
  - `Container.String()` - tree-like output
  - `Container.DumpFragments()` - detailed fragment dump
  - kfxdump tool uses new implementation

- [x] **1.6 Generate Skeleton** (`generate.go`)
  - `Generate()` creates minimal KFX container with basic metadata
  - Uses `misc.GetAppName()` and `misc.GetVersion()` for generator info
  - Uses `DefaultChunkSize` constant
  - Writes debug output to `.debug.txt` file
  - Logs elapsed time via defer
  - Write/read roundtrip verified with kfxdump

### Phase 2: Fragment Model (`convert/kfx/`)

- [ ] **2.1 Fragment Types** (`fragment.go`)
  - Define Fragment struct (ftype, fid, value)
  - Implement FragmentKey (single vs dual annotation)
  - Implement FragmentList with type/id indexing
  - Define ROOT_FRAGMENT_TYPES, SINGLETON_FRAGMENT_TYPES sets
  - Define REQUIRED_BOOK_FRAGMENT_TYPES, ALLOWED_BOOK_FRAGMENT_TYPES

- [ ] **2.2 Fragment Values** (`values.go`)
  - Helper types for common Ion value patterns (structs, lists, symbols)
  - Value builders for style properties, content nodes, navigation
  - Support for nested struct construction

### Phase 3: Core Fragment Generators (`convert/kfx/`)

- [ ] **3.1 Container Fragment** (`frag_container.go`) - $270
  - Generate container metadata fragment
  - Include container ID, version, generator info
  - Include entity list ($181)

- [ ] **3.2 Symbol Table Fragment** (`frag_symtab.go`) - $ion_symbol_table
  - Generate document symbol table
  - Include imports (YJ_symbols) and local symbols
  - Track symbols used during generation

- [ ] **3.3 Container Entity Map** (`frag_entitymap.go`) - $419
  - Generate entity map with container list ($252)
  - Optionally include entity dependencies ($253)

- [ ] **3.4 Metadata Fragments** (`frag_metadata.go`)
  - $258 (Metadata): title, author, language, etc.
  - $490 (BookMetadata): categorised metadata
  - $538 (DocumentData): reading orders

- [ ] **3.5 Format Capabilities** (`frag_capabilities.go`) - $593
  - Generate format capabilities for KFX v2
  - Include kfxgen capabilities (positionMaps, etc.)

### Phase 4: Content Fragment Generators (`convert/kfx/`)

- [ ] **4.1 Storyline Fragment** (`frag_storyline.go`) - $259
  - Generate storyline (root content container)
  - Map FB2 body structure to KFX storyline

- [ ] **4.2 Section Fragments** (`frag_section.go`) - $260
  - Generate sections for FB2 bodies/sections
  - Include section content with $155 (id) references

- [ ] **4.3 Content Elements** (`frag_content.go`)
  - Text content ($269) with style events ($142)
  - Paragraph blocks with inline content
  - Images ($271) with external resource references
  - Lists ($276, $277) - ordered/unordered
  - Tables ($278, $279) - rows/cells

- [ ] **4.4 Style Fragments** (`frag_style.go`) - $157
  - Generate style fragments from FB2 stylesheets
  - Map CSS properties to YJ property symbols
  - Handle font, color, spacing, alignment properties

### Phase 5: Resource & Navigation Fragments (`convert/kfx/`)

- [ ] **5.1 External Resources** (`frag_resource.go`) - $164
  - Generate resource descriptors for images
  - Include dimensions, format, MIME type

- [ ] **5.2 Raw Media** (`frag_rawmedia.go`) - $417
  - Generate raw media fragments for images
  - Store binary data as Ion BLOBs

- [ ] **5.3 Anchor Fragments** (`frag_anchor.go`) - $266
  - Generate anchor fragments for internal links
  - Include position references ($183)

- [ ] **5.4 Navigation Fragments** (`frag_navigation.go`)
  - $389 (BookNavigation): per reading order
  - $391 (NavContainer): TOC structure
  - $393 (NavUnit): individual nav entries
  - Note: Skip $390, $394 (magazine/conditional specific)

### Phase 6: Position Mapping (`convert/kfx/`)

- [ ] **6.1 Position Map** (`frag_posmap.go`) - $264
  - Generate EID to section membership map
  - Track element IDs during content generation

- [ ] **6.2 Position ID Map** (`frag_posidmap.go`) - $265
  - Generate PID to (EID, offset) mapping
  - Support flat map format (simpler for initial implementation)

- [ ] **6.3 Location Map** (`frag_locmap.go`) - $550
  - Generate location to position mapping
  - Used for Kindle "location" feature

### Phase 7: Debug & Verification (`convert/kfx/`, `cmd/debug/kfxdump/`)

- [ ] **7.1 Debug Output** (`debug.go`)
  - Implement String() method for KFX book structure
  - Tree-like text output similar to epub debug format
  - Show all fragments with types, IDs, and key properties

- [ ] **7.2 KFX Dump Tool** (`cmd/debug/kfxdump/main.go`)
  - Read KFX file using container parser
  - Print fragment list using debug output
  - Compare with reference KFX files from /mnt/d/test/

### Phase 8: Integration & Testing

- [ ] **8.1 Main Generate Function** (`generate.go`)
  - Orchestrate all fragment generators
  - Build complete KFX book from FB2 content
  - Write to output file

- [ ] **8.2 Validation Testing**
  - Run output through testdata/input.py
  - Fix any errors reported by KFXInput
  - Document acceptable warnings

- [ ] **8.3 Comparison Testing**
  - Compare debug output with reference KFX files
  - Verify structure matches working examples

## Files to Create

```
convert/kfx/
├── generate.go          # Main Generate function (update existing)
├── symbols.go           # Symbol table definitions
├── ionutil.go           # Ion helper utilities
├── container.go         # CONT format read/write
├── entity.go            # ENTY format read/write
├── fragment.go          # Fragment model
├── values.go            # Value construction helpers
├── debug.go             # Debug output
├── frag_container.go    # $270 Container
├── frag_symtab.go       # $ion_symbol_table
├── frag_entitymap.go    # $419 ContainerEntityMap
├── frag_metadata.go     # $258, $490, $538
├── frag_capabilities.go # $593 FormatCapabilities
├── frag_storyline.go    # $259 Storyline
├── frag_section.go      # $260 Section
├── frag_content.go      # Text, Image, List, Table content
├── frag_style.go        # $157 Style
├── frag_resource.go     # $164 ExternalResource
├── frag_rawmedia.go     # $417 RawMedia
├── frag_anchor.go       # $266 Anchor
├── frag_navigation.go   # $389, $391, $393 Navigation
├── frag_posmap.go       # $264 PositionMap
├── frag_posidmap.go     # $265 PositionIDMap
├── frag_locmap.go       # $550 LocationMap
└── generate_test.go     # Unit tests

cmd/debug/kfxdump/
└── main.go              # KFX dump utility (update existing)
```

## Symbol Categories Reference

Key symbol ranges from docs/symdict.md:
- $10-$155: Style/formatting properties
- $156-$200: Content/document structure
- $212-$260: Navigation, metadata
- $269-$282: Content types (text, image, list, table, etc.)
- $284-$287: Format types (PNG, JPG, GIF)
- $306-$395: Units, conditions, more navigation
- $409-$419: Container-specific symbols
- $538-$611: Advanced features (document data, position maps)
- $756-$770: Ruby/annotation content

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
- May need to work around strict typing in some places
- BLOBs for raw media are straightforward

### Validation Expectations
- Must pass KFXInput validation with no errors
- Some warnings may be acceptable (document which ones)
- Compare structure with reference files in /mnt/d/test/
