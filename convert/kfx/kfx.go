package kfx

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
)

// Generate creates the KFX output file.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	log.Info("Generating KFX", zap.String("output", outputPath))

	return fmt.Errorf("KFX generation not yet implemented")
}
