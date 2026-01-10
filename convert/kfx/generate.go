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
	contentFragments, nextEID, sectionNames, tocEntries, sectionEIDs, idToEID, err := generateStoryline(c.Book, styles, imageResourceInfo, startEID, c.FootnotesIndex)
	if err != nil {
		return err
	}

	annotationEnabled := cfg.Annotation.Enable && c.Book.Description.TitleInfo.Annotation != nil
	tocPageEnabled := cfg.TOCPage.Placement != common.TOCPagePlacementNone
	if annotationEnabled || tocPageEnabled {
		sectionNames, tocEntries, sectionEIDs, nextEID, err = addGeneratedSections(c, cfg, styles, contentFragments, sectionNames, tocEntries, sectionEIDs, nextEID, log)
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

	// Add style fragments from registry
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

	// $538 DocumentData - reading orders with sections
	docDataFrag := BuildDocumentData(sectionNames)
	if err := container.Fragments.Add(docDataFrag); err != nil {
		return err
	}

	// Build navigation containers (TOC) from TOC entries
	if len(tocEntries) > 0 {
		navFrag := BuildNavigation(tocEntries, nextEID)
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
	if err := container.Fragments.Add(BuildLocationMap(allEIDs)); err != nil {
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

	// $593 FormatCapabilities - keep minimal (KFXInput reads kfxgen.* here)
	container.FormatCapabilities = BuildFormatCapabilities(DefaultFormatFeatures()).Value

	// $585 content_features - reflow/canonical features live here in reference files
	maxSectionPIDCount := computeMaxSectionPIDCount(sectionEIDs, posItems)
	reflowSectionSize := reflowSectionSizeVersion(maxSectionPIDCount)
	if err := container.Fragments.Add(BuildContentFeatures(reflowSectionSize)); err != nil {
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

	// Combine all stylesheet data
	var combinedCSS []byte
	for _, sheet := range stylesheets {
		if sheet.Type != "" && sheet.Type != "text/css" {
			continue
		}
		if sheet.Data != "" {
			combinedCSS = append(combinedCSS, []byte(sheet.Data)...)
			combinedCSS = append(combinedCSS, '\n')
		}
	}

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
