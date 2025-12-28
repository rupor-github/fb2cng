package kfx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"math/bits"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

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
	// Create style registry with default styles
	styles := DefaultStyleRegistry()

	usedIDs := collectUsedImageIDs(c.Book)
	usedImages := make(fb2.BookImages, len(usedIDs))
	for id, img := range c.ImagesIndex {
		if usedIDs[id] {
			usedImages[id] = img
		}
	}

	externalRes, rawMedia, imageResourceNames := buildImageResourceFragments(usedImages)

	// Generate storyline and section fragments from book content
	// EIDs start at 1000 - this is arbitrary but leaves room for future system IDs
	startEID := 1000
	contentFragments, nextEID, sectionNames, tocEntries, sectionEIDs, err := GenerateStorylineFromBook(c.Book, styles, imageResourceNames, startEID)
	if err != nil {
		return err
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
	if len(c.Book.Description.TitleInfo.Coverpage) > 0 && imageResourceNames != nil {
		coverID := strings.TrimPrefix(c.Book.Description.TitleInfo.Coverpage[0].Href, "#")
		if rn, ok := imageResourceNames[coverID]; ok {
			coverResName = rn
		}
	}

	// $490 Book Metadata - categorised metadata (title, author, language, etc.)
	bookMetadataFrag := BuildBookMetadataFragment(c, cfg, log, container.ContainerID, coverResName)
	if err := container.Fragments.Add(bookMetadataFrag); err != nil {
		return err
	}

	// $258 Metadata - reading orders only
	metadataFrag := BuildMetadataFragment(sectionNames)
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
	docDataFrag := BuildDocumentDataFragment(sectionNames)
	if err := container.Fragments.Add(docDataFrag); err != nil {
		return err
	}

	// Build navigation containers (TOC) from TOC entries
	if len(tocEntries) > 0 {
		navFrag := BuildNavigationFragment(tocEntries, nextEID)
		if err := container.Fragments.Add(navFrag); err != nil {
			return err
		}
	}
	// $395 resource_path (present in reference output; usually empty)
	if err := container.Fragments.Add(BuildResourcePathFragment()); err != nil {
		return err
	}

	// Phase 6: Position maps ($264/$265) + location map ($550)
	if err := container.Fragments.Add(BuildPositionMapFragment(sectionNames, sectionEIDs)); err != nil {
		return err
	}
	if err := container.Fragments.Add(BuildPositionIdMapFragment(allEIDs, posItems)); err != nil {
		return err
	}
	if err := container.Fragments.Add(BuildLocationMapFragment(allEIDs)); err != nil {
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
	referencedAnchors := make(map[string]bool, len(c.LinksRevIndex))
	for id := range c.LinksRevIndex {
		referencedAnchors[id] = true
	}
	for _, frag := range buildAnchorFragments(tocEntries, referencedAnchors) {
		if err := container.Fragments.Add(frag); err != nil {
			return err
		}
	}

	// $593 FormatCapabilities - keep minimal (KFXInput reads kfxgen.* here)
	container.FormatCapabilities = BuildFormatCapabilitiesFragment(DefaultFormatFeatures()).Value

	// $585 content_features - reflow/canonical features live here in reference files
	maxSectionPIDCount := computeMaxSectionPIDCount(sectionEIDs, posItems)
	reflowSectionSize := reflowSectionSizeVersion(maxSectionPIDCount)
	if err := container.Fragments.Add(BuildContentFeaturesFragment(reflowSectionSize)); err != nil {
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
	entityMapFrag := BuildContainerEntityMapFragment(container.ContainerID, container.Fragments, deps)
	if err := container.Fragments.Add(entityMapFrag); err != nil {
		return err
	}

	log.Debug("Built fragments", zap.Int("count", container.Fragments.Len()))

	return nil
}

const charsetCR = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// reflowSectionSizeVersion computes the reflow-section-size major version from
// the maximum per-section PID count. This value is stored in $585 content_features
// and used by KFXInput for validation.
//
// The formula approximates: version = max(1, ceil(log2(maxCount)) - 11)
// Using bits.Len(n-1) gives ceil(log2(n)) for n > 0.
func reflowSectionSizeVersion(maxSectionPIDCount int) int {
	if maxSectionPIDCount <= 0 {
		return 1
	}
	v := bits.Len(uint(maxSectionPIDCount - 1))
	ver := min(v-11, 1)
	return ver
}

func computeMaxSectionPIDCount(sectionEIDs map[string][]int, posItems []PositionItem) int {
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
