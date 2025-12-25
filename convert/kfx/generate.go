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
	"golang.org/x/text/language"

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
// This is a minimal skeleton that will be expanded as fragment generators are implemented.
func buildFragments(container *Container, c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) error {
	// TODO: Phase 2-6 - Add fragment generators here

	// For now, create minimal required fragments

	// $258 Metadata - basic book metadata
	metadata := NewStruct()
	metadata.Set(SymTitle, c.Book.Description.TitleInfo.BookTitle.Value) // title ($153)

	if len(c.Book.Description.TitleInfo.Authors) > 0 {
		author := c.Book.Description.TitleInfo.Authors[0]
		authorName := ""
		if author.FirstName != "" {
			authorName = author.FirstName
		}
		if author.LastName != "" {
			if authorName != "" {
				authorName += " "
			}
			authorName += author.LastName
		}
		if authorName != "" {
			metadata.Set(SymAuthor, authorName) // author ($222)
		}
	}
	if lang := c.Book.Description.TitleInfo.Lang; lang != language.Und {
		metadata.Set(SymLanguage, lang.String()) // language ($10)
	}

	container.Fragments.Add(&Fragment{
		FType: SymMetadata, // $258
		FID:   SymMetadata,
		Value: metadata,
	})

	// $538 DocumentData - reading orders
	docData := NewStruct()
	docData.Set(SymReadingOrders, ListValue{ // reading_orders ($169)
		NewStruct().Set(SymUniqueID, "default"), // id ($155)
	})

	container.Fragments.Add(&Fragment{
		FType: SymDocumentData, // $538
		FID:   SymDocumentData,
		Value: docData,
	})

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
