package convert

import (
	"context"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/convert/epub"
	"fbc/convert/kfx"
)

// WriteTo generates output in the specified format and writes it to the destination.
func (c *Content) WriteTo(ctx context.Context, format config.OutputFmt, outputPath string, log *zap.Logger) error {
	switch format {
	case config.OutputFmtEpub2, config.OutputFmtEpub3, config.OutputFmtKepub:
		return epub.New(format, log).Generate(ctx, c, outputPath)
	case config.OutputFmtKfx:
		return kfx.New(log).Generate(ctx, c, outputPath)
	default:
		return nil
	}
}
