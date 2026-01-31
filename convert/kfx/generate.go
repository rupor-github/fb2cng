package kfx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
	"fbc/misc"
)

// Generate creates the KFX output file.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}

	log.Info("KFX generation starting", zap.String("output", outputPath))
	defer func(start time.Time) {
		if err == nil {
			log.Info("KFX generation completed", zap.Duration("elapsed", time.Since(start)))
		}
	}(time.Now())

	// Generate container ID from document ID
	containerID := "CR!" + hashToAlphanumeric(c.Book.Description.DocumentInfo.ID, 28)

	// Create container with basic metadata
	container := NewContainer()
	container.ContainerID = containerID
	container.GeneratorApp = misc.GetAppName()
	container.GeneratorPkg = misc.GetVersion()

	// Build minimal fragments from content
	if err := buildFragments(container, c, cfg, log); err != nil {
		return err
	}

	// Serialize container to KFX
	kfxData, err := container.WriteContainer()
	if err != nil {
		return err
	}

	// Write debug output when in debug mode (after serialization so all data is populated)
	// Use c.WorkDir (which is already a temp dir) so it is captured by the reporting archive.
	if c.Debug {
		debugPath := filepath.Join(c.WorkDir, filepath.Base(outputPath)+".debug.txt")
		debugOutput := container.String() + "\n\n" + container.DumpFragments()
		if err := os.WriteFile(debugPath, []byte(debugOutput), 0644); err != nil {
			log.Warn("Failed to write debug output", zap.Error(err))
		}
	}

	// Ensure output directory exists
	if dir := filepath.Dir(outputPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Write KFX file
	if err := os.WriteFile(outputPath, kfxData, 0644); err != nil {
		return err
	}

	log.Debug("KFX written",
		zap.String("output", outputPath),
		zap.Int("size", len(kfxData)),
		zap.Int("fragments", container.Fragments.Len()),
	)

	return nil
}

// buildFragments creates KFX fragments from content.
func buildFragments(container *Container, c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) error {
	// Create style tracer if debug mode is enabled
	var tracer *StyleTracer
	if c.Debug {
		tracer = NewStyleTracer(c.WorkDir)
	}

	// Create style registry from CSS stylesheets
	styles := buildStyleRegistry(c.Book.Stylesheets, tracer, log)

	usedIDs := collectUsedImageIDs(c.Book)
	usedImages := make(fb2.BookImages, len(usedIDs))
	for id, img := range c.ImagesIndex {
		if usedIDs[id] {
			usedImages[id] = img
		}
	}

	externalRes, rawMedia, imageResourceInfo := buildImageResourceFragments(usedImages)

	// Generate storyline and section fragments from book content
	// EIDs start at 1000 - this is arbitrary but leaves room for future system IDs
	startEID := 1000
	contentFragments, nextEID, sectionNames, tocEntries, sectionEIDs, idToEID, landmarks, err := generateStoryline(c, styles, imageResourceInfo, startEID)
	if err != nil {
		return err
	}

	annotationEnabled := cfg.Annotation.Enable && c.Book.Description.TitleInfo.Annotation != nil
	tocPageEnabled := cfg.TOCPage.Placement != common.TOCPagePlacementNone
	if annotationEnabled || tocPageEnabled {
		sectionNames, tocEntries, sectionEIDs, nextEID, landmarks, idToEID, err = addGeneratedSections(c, cfg, styles, contentFragments, sectionNames, tocEntries, sectionEIDs, nextEID, landmarks, idToEID, imageResourceInfo, log)
		if err != nil {
			return err
		}
	}

	posItems := CollectPositionItems(contentFragments, sectionNames)
	allEIDs := CollectAllEIDs(sectionEIDs)
	// Prefer actual reading-order EIDs when available.
	if len(posItems) > 0 {
		allEIDs = make([]int, 0, len(posItems))
		for _, it := range posItems {
			allEIDs = append(allEIDs, it.EID)
		}
	}

	// Cover image resource name (e.g. "e6") for metadata.
	coverResName := ""
	if len(c.Book.Description.TitleInfo.Coverpage) > 0 && imageResourceInfo != nil {
		coverID := strings.TrimPrefix(c.Book.Description.TitleInfo.Coverpage[0].Href, "#")
		if info, ok := imageResourceInfo[coverID]; ok {
			coverResName = info.ResourceName
		}
	}

	// $490 Book Metadata - categorised metadata (title, author, language, etc.)
	bookMetadataFrag := BuildBookMetadata(c, cfg, container.ContainerID, coverResName, log)
	if err := container.Fragments.Add(bookMetadataFrag); err != nil {
		return err
	}

	// $258 Metadata - reading orders only
	metadataFrag := BuildMetadata(sectionNames)
	if err := container.Fragments.Add(metadataFrag); err != nil {
		return err
	}

	// Recompute which styles are actually used by scanning content fragments.
	// This must be done after all content is generated (including margin collapsing
	// which may create new style variants and orphan old ones).
	styles.RecomputeUsedStyles(contentFragments)

	// Add style fragments from registry (only used styles are included)
	for _, styleFrag := range styles.BuildFragments() {
		if err := container.Fragments.Add(styleFrag); err != nil {
			return err
		}
	}

	// Add all content fragments to container
	for _, frag := range contentFragments.All() {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}

	// $538 DocumentData - reading orders with sections + global defaults
	maxID := nextEID - 1
	docDataFrag := BuildDocumentData(sectionNames, maxID)
	if err := container.Fragments.Add(docDataFrag); err != nil {
		return err
	}

	// Build navigation containers (TOC + landmarks + APPROXIMATE_PAGE_LIST) from TOC entries
	if len(tocEntries) > 0 {
		// Pass page size from config if page map is enabled
		pageSize := 0
		if cfg.PageMap.Enable {
			pageSize = cfg.PageMap.Size
		}
		navFrag := BuildNavigation(tocEntries, nextEID, posItems, pageSize, cfg.TOCPage.ChaptersWithoutTitle, landmarks)
		if err := container.Fragments.Add(navFrag); err != nil {
			return err
		}
	}
	// $395 resource_path (present in reference output; usually empty)
	if err := container.Fragments.Add(BuildResourcePath()); err != nil {
		return err
	}

	// Phase 6: Position maps ($264/$265) + location map ($550)
	if err := container.Fragments.Add(BuildPositionMap(sectionNames, sectionEIDs)); err != nil {
		return err
	}
	if err := container.Fragments.Add(BuildPositionIDMap(allEIDs, posItems)); err != nil {
		return err
	}
	if err := container.Fragments.Add(BuildLocationMap(posItems)); err != nil {
		return err
	}

	// $164 External resources + $417 Raw media (images)
	for _, frag := range externalRes {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}
	for _, frag := range rawMedia {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}

	// $266 Anchors (for internal links)
	referencedAnchors := collectLinkTargets(contentFragments)
	for _, frag := range buildAnchorFragments(idToEID, referencedAnchors) {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}

	// $266 Anchors (for external links - URLs with anchor_name + uri)
	for _, frag := range styles.BuildExternalLinkFragments() {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}

	// $593 FormatCapabilities - keep minimal (KFXInput reads kfxgen.* here)
	// Include pidMapWithOffset when inline images use offset-based position entries
	formatFeatures := DefaultFormatFeatures()
	if HasInlineImages(posItems) {
		formatFeatures = FormatFeaturesWithPIDMapOffset()
	}
	container.FormatCapabilities = BuildFormatCapabilities(formatFeatures).Value

	// $585 content_features - reflow/canonical features live here in reference files
	// Collect content feature info to conditionally add features (matching KP3 behavior)
	maxSectionPIDCount := computeMaxSectionPIDCount(sectionEIDs, posItems)
	contentFeatureInfo := collectContentFeatureInfo(c, imageResourceInfo, maxSectionPIDCount)
	if err := container.Fragments.Add(BuildContentFeatures(contentFeatureInfo)); err != nil {
		return err
	}

	// $597 auxiliary_data - reference uses this to mark target sections.
	for _, frag := range BuildAuxiliaryDataFragments(sectionNames) {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}

	// $419 ContainerEntityMap - must be added after all other fragments
	deps := ComputeEntityDependencies(container.Fragments)
	entityMapFrag := BuildContainerEntityMap(container.ContainerID, container.Fragments, deps)
	if err := container.Fragments.Add(entityMapFrag); err != nil {
		return err
	}

	// Flush style trace if enabled
	if tracer != nil {
		if tracePath := tracer.Flush(); tracePath != "" {
			log.Debug("Style trace written", zap.String("path", tracePath))
		}
	}

	log.Debug("Built fragments", zap.Int("count", container.Fragments.Len()))

	return nil
}

// reflowSectionSizeVersion computes the reflow-section-size major version from
// the maximum per-section PID count.
//
// This matches KFXInput's calculation:
//
//	if max > 65536:
//	  v = min(((max-65536)//16384)+2, 256)
//	else:
//	  v = 1
func reflowSectionSizeVersion(maxSectionPIDCount int) int {
	if maxSectionPIDCount <= 65536 {
		return 1
	}
	ver := ((maxSectionPIDCount - 65536) / (16 * 1024)) + 2
	return min(ver, 256)
}

func computeMaxSectionPIDCount(sectionEIDs sectionEIDsBySectionName, posItems []PositionItem) int {
	eidToSection := make(map[int]string)
	for sec, eids := range sectionEIDs {
		for _, eid := range eids {
			eidToSection[eid] = sec
		}
	}

	bySection := make(map[string]int)
	for _, it := range posItems {
		sec := eidToSection[it.EID]
		bySection[sec] += it.Length
	}

	max := 0
	for _, v := range bySection {
		if v > max {
			max = v
		}
	}
	return max
}

// collectContentFeatureInfo gathers information about the content to determine
// which features should be included in content_features ($585).
// This matches KP3 behavior where features are conditionally added based on content.
func collectContentFeatureInfo(c *content.Content, imageResources imageResourceInfoByID, maxSectionPIDCount int) *ContentFeatureInfo {
	info := &ContentFeatureInfo{
		ReflowSectionSize: reflowSectionSizeVersion(maxSectionPIDCount),
		Language:          c.Book.Description.TitleInfo.Lang.String(),
	}

	// Check for tables and collect max image dimensions
	hasTables, hasTableWithLinks := checkForTables(c.Book)
	info.HasTables = hasTables
	info.HasTableWithLinks = hasTableWithLinks

	// Find max image dimensions for HDV detection
	for _, res := range imageResources {
		if res.Width > info.MaxImageWidth {
			info.MaxImageWidth = res.Width
		}
		if res.Height > info.MaxImageHeight {
			info.MaxImageHeight = res.Height
		}
	}

	return info
}

// checkForTables scans the book content for tables and checks if any have links.
func checkForTables(book *fb2.FictionBook) (hasTables, hasTableWithLinks bool) {
	// Check all bodies for tables
	for i := range book.Bodies {
		if checkSectionsForTables(book.Bodies[i].Sections, &hasTableWithLinks) {
			hasTables = true
		}
	}
	return hasTables, hasTableWithLinks
}

// checkSectionsForTables recursively checks sections for tables.
func checkSectionsForTables(sections []fb2.Section, hasTableWithLinks *bool) bool {
	hasTables := false
	for i := range sections {
		section := &sections[i]
		// Check section content for tables
		for _, item := range section.Content {
			if item.Kind == fb2.FlowTable && item.Table != nil {
				hasTables = true
				// Check if table has links
				if !*hasTableWithLinks && tableHasLinks(item.Table) {
					*hasTableWithLinks = true
				}
			}
			// Recursively check nested sections
			if item.Kind == fb2.FlowSection && item.Section != nil {
				if checkSectionForTables(item.Section, hasTableWithLinks) {
					hasTables = true
				}
			}
		}
	}
	return hasTables
}

// checkSectionForTables recursively checks a single section for tables.
func checkSectionForTables(section *fb2.Section, hasTableWithLinks *bool) bool {
	hasTables := false
	for _, item := range section.Content {
		if item.Kind == fb2.FlowTable && item.Table != nil {
			hasTables = true
			if !*hasTableWithLinks && tableHasLinks(item.Table) {
				*hasTableWithLinks = true
			}
		}
		// Recursively check nested sections
		if item.Kind == fb2.FlowSection && item.Section != nil {
			if checkSectionForTables(item.Section, hasTableWithLinks) {
				hasTables = true
			}
		}
	}
	return hasTables
}

// tableHasLinks checks if a table contains any links in its cells.
func tableHasLinks(table *fb2.Table) bool {
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if cellHasLinks(cell.Content) {
				return true
			}
		}
	}
	return false
}

// cellHasLinks checks if cell content contains any links.
func cellHasLinks(content []fb2.InlineSegment) bool {
	for _, seg := range content {
		if segmentHasLinks(&seg) {
			return true
		}
	}
	return false
}

// segmentHasLinks recursively checks if an inline segment contains links.
func segmentHasLinks(seg *fb2.InlineSegment) bool {
	if seg.Kind == fb2.InlineLink {
		return true
	}
	for i := range seg.Children {
		if segmentHasLinks(&seg.Children[i]) {
			return true
		}
	}
	return false
}

func collectLinkTargets(fragments *FragmentList) map[string]bool {
	targets := make(map[string]bool)

	// Helper function to recursively collect link targets from entries
	var collectFromEntries func(entries []any)
	collectFromEntries = func(entries []any) {
		for _, it := range entries {
			entry, ok := it.(StructValue)
			if !ok {
				continue
			}

			// Check for nested content_list (wrapper containers)
			if nestedEntries, ok := entry.GetList(SymContentList); ok && len(nestedEntries) > 0 {
				collectFromEntries(nestedEntries)
			}

			evs, ok := entry.GetList(SymStyleEvents)
			if !ok {
				continue
			}
			for _, evAny := range evs {
				ev, ok := evAny.(StructValue)
				if !ok {
					continue
				}
				linkTo, ok := ev[SymLinkTo]
				if !ok {
					continue
				}
				switch x := linkTo.(type) {
				case SymbolByNameValue:
					if s := string(x); s != "" {
						targets[s] = true
					}
				case string:
					if x != "" {
						targets[x] = true
					}
				}
			}
		}
	}

	for _, frag := range fragments.GetByType(SymStoryline) {
		v, ok := frag.Value.(StructValue)
		if !ok {
			continue
		}
		entries, ok := v.GetList(SymContentList)
		if !ok {
			continue
		}
		collectFromEntries(entries)
	}
	return targets
}

// buildStyleRegistry creates a style registry from CSS stylesheets.
// It parses the CSS, converts rules to KFX style definitions, and registers them.
// Falls back to default styles if no stylesheets are provided.
// If tracer is non-nil, style operations will be logged to the trace.
func buildStyleRegistry(stylesheets []fb2.Stylesheet, tracer *StyleTracer, log *zap.Logger) *StyleRegistry {
	// If no stylesheets, return defaults
	if len(stylesheets) == 0 {
		log.Debug("No stylesheets provided, using defaults only")
		sr := DefaultStyleRegistry()
		sr.SetTracer(tracer)
		return sr
	}

	log.Debug("Processing stylesheets", zap.Int("count", len(stylesheets)))

	// Combine all stylesheet data
	var combinedCSS []byte
	for i, sheet := range stylesheets {
		log.Debug("Stylesheet entry",
			zap.Int("index", i),
			zap.String("type", sheet.Type),
			zap.Int("data_len", len(sheet.Data)))
		if sheet.Type != "" && sheet.Type != "text/css" {
			continue
		}
		if sheet.Data != "" {
			combinedCSS = append(combinedCSS, []byte(sheet.Data)...)
			combinedCSS = append(combinedCSS, '\n')
		}
	}

	log.Debug("Combined CSS", zap.Int("total_bytes", len(combinedCSS)))

	if len(combinedCSS) == 0 {
		log.Debug("No CSS data in stylesheets, using defaults only")
		sr := DefaultStyleRegistry()
		sr.SetTracer(tracer)
		return sr
	}

	// Create registry from CSS
	registry, warnings := NewStyleRegistryFromCSS(combinedCSS, tracer, log)

	// Log warnings at debug level
	for _, w := range warnings {
		log.Debug("CSS conversion warning", zap.String("warning", w))
	}

	log.Info("CSS stylesheets processed",
		zap.Int("stylesheets", len(stylesheets)),
		zap.Int("warnings", len(warnings)))

	return registry
}

const charsetCR = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomAlphanumeric generates a random string of the given length
// containing only uppercase Latin letters (A-Z) and digits (0-9).
func randomAlphanumeric(length int) string {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charsetCR)))

	for i := range length {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			panic(err)
		}
		result[i] = charsetCR[num.Int64()]
	}
	return string(result)
}

// hashToAlphanumeric hashes the input string to produce a deterministic string of exactly
// `length` bytes containing only uppercase Latin letters (A-Z) and digits (0-9).
// If the input is empty, a random string is generated instead.
func hashToAlphanumeric(input string, length int) string {
	if input == "" {
		return randomAlphanumeric(length)
	}
	if length <= 0 {
		return ""
	}

	hash := sha256.Sum256([]byte(input))
	result := make([]byte, length)
	for i := range length {
		// SHA-256 gives 32 bytes; we only currently need 28/32. For any larger length,
		// repeat hash bytes deterministically.
		hb := hash[i%len(hash)]
		idx := hb % byte(len(charsetCR))
		result[i] = charsetCR[idx]
	}
	return string(result)
}
