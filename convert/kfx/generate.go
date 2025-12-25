package kfx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
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
	containerID := "CR!" + hashTo28Alphanumeric(c.Book.Description.DocumentInfo.ID)

	// Create container with basic metadata
	container := &Container{
		ContainerID:     containerID,
		CompressionType: 0, // No compression
		DRMScheme:       0, // No DRM
		ChunkSize:       DefaultChunkSize,
		GeneratorApp:    misc.GetAppName(),
		GeneratorPkg:    misc.GetVersion(),
		Fragments:       NewFragmentList(),
	}

	// Build minimal fragments from content
	if err := buildFragments(container, c, cfg, log); err != nil {
		return err
	}

	// Write debug output when in debug mode
	if c.Debug {
		debugPath := filepath.Join(c.WorkDir, filepath.Base(outputPath)+".debug.txt")
		debugOutput := container.String() + "\n" + container.DumpFragments()
		if err := os.WriteFile(debugPath, []byte(debugOutput), 0644); err != nil {
			log.Warn("Failed to write debug output", zap.Error(err))
		}
	}

	// Serialize container to KFX
	kfxData, err := container.WriteContainer()
	if err != nil {
		return err
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
	// $258 Metadata - basic book metadata
	metadataFrag := BuildMetadataFragment(c, cfg, log)
	if err := container.Fragments.Add(metadataFrag); err != nil {
		return err
	}

	// $538 DocumentData - reading orders (required for KFX v2)
	docDataFrag := BuildDocumentDataFragmentSimple("default")
	if err := container.Fragments.Add(docDataFrag); err != nil {
		return err
	}

	// $593 FormatCapabilities - KFX v2 format capabilities
	fcFrag := BuildFormatCapabilitiesFragment(nil) // Use default features
	if err := container.Fragments.Add(fcFrag); err != nil {
		return err
	}

	// Store format capabilities in container for serialization
	container.FormatCapabilities = fcFrag.Value

	// $419 ContainerEntityMap - must be added after all other fragments
	entityMapFrag := BuildContainerEntityMapFragment(container.ContainerID, container.Fragments)
	if err := container.Fragments.Add(entityMapFrag); err != nil {
		return err
	}

	log.Debug("Built fragments", zap.Int("count", container.Fragments.Len()))

	return nil
}

const charsetCR = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomAlphanumeric28 generates a random string of exactly 28 bytes
// containing only uppercase Latin letters (A-Z) and digits (0-9).
func randomAlphanumeric28() string {
	const length = 28

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

// hashTo28Alphanumeric hashes the input string to produce exactly 28 bytes
// containing only uppercase Latin letters (A-Z) and digits (0-9). It does this
// deterministically - same input string will always produce the same output
// string. If the input is empty, a random string is generated instead.
func hashTo28Alphanumeric(input string) string {
	if input == "" {
		return randomAlphanumeric28()
	}

	// Use SHA-256 to hash the input
	hash := sha256.Sum256([]byte(input))

	// Map hash bytes to full alphanumeric charset (A-Z, 0-9)
	result := make([]byte, 28)

	for i := range 28 {
		// Use hash bytes to deterministically select from charset
		idx := hash[i] % byte(len(charsetCR))
		result[i] = charsetCR[idx]
	}

	return string(result)
}
