package kfx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
)

// Generate creates the KFX output file.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// if true {
	// 	return fmt.Errorf("KFX generation is experimental and not yet implemented")
	// }

	log.Info("Generating KFX", zap.String("output", outputPath))

	containerID := "CR!" + hashTo28Alphanumeric(c.Book.Description.DocumentInfo.ID)
	_ = containerID

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
