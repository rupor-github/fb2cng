package fb2

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"mime"
	"path"
	"strings"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"fbc/common"
	"fbc/config"
	"fbc/jpegquality"
	imgutil "fbc/utils/images"
)

// Image processing functions for FictionBook.

// PrepareImages processes all binary objects in the FictionBook creating
// actual image and building image index. Never returns an error - uses placeholder for broken images.
// Non-image binaries (e.g., fonts) are skipped.
func (fb *FictionBook) PrepareImages(kindle bool, cfg *config.ImagesConfig, log *zap.Logger) BookImages {
	index := make(BookImages)

	imgNum := 1
	for i := range fb.Binaries {
		// Skip non-image binaries (e.g., fonts loaded by stylesheet normalization)
		if !isImageMIME(fb.Binaries[i].ContentType) {
			log.Debug("Skipping non-image binary",
				zap.String("id", fb.Binaries[i].ID),
				zap.String("content-type", fb.Binaries[i].ContentType))
			continue
		}

		if _, exists := index[fb.Binaries[i].ID]; exists {
			log.Debug("Duplicate binary ID found, skipping", zap.String("id", fb.Binaries[i].ID))
			continue
		}
		cover := len(fb.Description.TitleInfo.Coverpage) > 0 && strings.HasSuffix(fb.Description.TitleInfo.Coverpage[0].Href, fb.Binaries[i].ID)
		bi := fb.Binaries[i].PrepareImage(kindle, cover, cfg, log)
		ext := mimeToExt(bi.MimeType)
		bi.Filename = path.Join(ImagesDir, fmt.Sprintf("img%05d.%s", imgNum, ext))
		imgNum++
		index[fb.Binaries[i].ID] = bi
	}
	return index
}

// handleImageError is a unified error handler for all image processing failures.
// It logs the error and optionally substitutes the image with a placeholder.
func (bo *BinaryObject) handleImageError(bi *BookImage, operation string, err error, kindle bool, cfg *config.ImagesConfig, log *zap.Logger) *BookImage {
	// Log warning with appropriate context
	if err != nil {
		log.Warn("Unable to "+operation+" image", zap.String("id", bo.ID), zap.String("content-type", bo.ContentType), zap.Error(err))
	} else {
		log.Warn("Unable to "+operation+" image", zap.String("id", bo.ID), zap.String("content-type", bo.ContentType))
	}

	if cfg.UseBroken {
		return bi
	}

	log.Debug("Substituting image with broken placeholder", zap.String("id", bo.ID))
	bi.Data = brokenImage
	bi.MimeType = "image/svg+xml"

	if !kindle {
		return bi
	}

	// brokenImage placeholder SVG already has thick strokes (stroke-width="4"),
	// no scaling needed
	img, rasterErr := imgutil.RasterizeSVGToImage(brokenImage, 0, 0, 0)
	if rasterErr != nil {
		log.Warn("Unable to rasterize broken placeholder SVG", zap.String("id", bo.ID), zap.Error(rasterErr))
		return bi
	}

	data, encErr := bo.encodeImage(img, "jpeg", cfg, log)
	if encErr != nil {
		log.Warn("Unable to encode rasterized broken placeholder", zap.String("id", bo.ID), zap.Error(encErr))
		return bi
	}

	bi.Data = data
	bi.MimeType = "image/jpeg"
	bi.Dim.Width = img.Bounds().Dx()
	bi.Dim.Height = img.Bounds().Dy()
	return bi
}

func (bo *BinaryObject) encodeImage(img image.Image, imgType string, cfg *config.ImagesConfig, log *zap.Logger) ([]byte, error) {
	var buf = new(bytes.Buffer)
	var err error

	switch imgType {
	case "png":
		err = imaging.Encode(buf, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
		if err != nil {
			return nil, fmt.Errorf("unable to encode processed PNG, ID - %s: %w", bo.ID, err)
		}
		return buf.Bytes(), nil
	case "jpeg":
		if cfg.Optimize && imgutil.IsGrayscale(img) {
			if _, ok := img.(*image.Gray); !ok {
				gray := image.NewGray(img.Bounds())
				draw.Draw(gray, gray.Bounds(), img, img.Bounds().Min, draw.Src)
				img = gray
			}
		}

		err = imaging.Encode(buf, img, imaging.JPEG, imaging.JPEGQuality(cfg.JPEGQuality))
		if err != nil {
			return nil, fmt.Errorf("unable to encode processed JPEG, ID - %s: %w", bo.ID, err)
		}
		data, added, err := imgutil.EnsureJFIFAPP0(buf.Bytes(), imgutil.DpiPxPerInch, 300, 300)
		if err != nil {
			return nil, fmt.Errorf("unable to insert jpeg JFIF APP0 marker segment, ID - %s: %w", bo.ID, err)
		}
		if added {
			log.Debug("Inserting jpeg JFIF APP0 marker segment", zap.String("id", bo.ID))
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported image format %q, ID - %s", imgType, bo.ID)
	}
}

// PrepareImage performs required image modifications leaving original data
// intact if no changes where requested. If image is decodable it will always
// attempt to normalize mime type. Never returns an error - uses placeholder
// for broken images.
// NOTE: today KP3 always seems to scale images to 2048 width and height, we do
// not do it
func (bo *BinaryObject) PrepareImage(kindle, cover bool, cfg *config.ImagesConfig, log *zap.Logger) *BookImage {

	bi := &BookImage{
		MimeType: bo.ContentType,
		Data:     bo.Data,
	}

	// SVG handling
	if strings.HasSuffix(strings.ToLower(bo.ContentType), "svg+xml") {
		bi.MimeType = "image/svg+xml"
		if !kindle {
			return bi
		}

		// Rasterize SVG - as far as I could tell KP3 always scales SVG to 2048
		// (DPI 140) keeping aspect ratio, we will use screen width instead.
		targetW, targetH := cfg.Screen.Width, 0

		// For our embedded not-found placeholder keep its intrinsic size
		// similar to how we handle broke images.
		if bytes.Equal(bo.Data, notFoundImage) {
			targetW, targetH = 0, 0
		}

		// Apply stroke width scaling only for builtin vignettes (thin strokes that need
		// to be visible on Kindle's high-resolution display). External file vignettes
		// are assumed to have appropriate stroke widths already.
		var strokeFactor float64
		if bo.BuiltinVignette {
			strokeFactor = imgutil.KindleSVGStrokeWidthFactor
		}
		img, err := imgutil.RasterizeSVGToImage(bo.Data, targetW, targetH, strokeFactor)
		if err != nil {
			return bo.handleImageError(bi, "rasterize", err, kindle, cfg, log)
		}

		bi.Dim.Width = img.Bounds().Dx()
		bi.Dim.Height = img.Bounds().Dy()
		imgType := "jpeg"
		bi.MimeType = "image/jpeg"

		// Cover resizing for SVG follows the same rules as raster images.
		if cover {
			w, h := cfg.Screen.Width, cfg.Screen.Height
			switch cfg.Cover.Resize {
			case common.ImageResizeModeNone:
			case common.ImageResizeModeKeepAR:
				if img.Bounds().Dy() < h {
					resizedImg := imaging.Resize(img, 0, h, imaging.Lanczos)
					if resizedImg == nil {
						return bo.handleImageError(bi, "resize", nil, kindle, cfg, log)
					}
					img = resizedImg
					bi.Dim.Width = img.Bounds().Dx()
					bi.Dim.Height = img.Bounds().Dy()
				}
			case common.ImageResizeModeStretch:
				resizedImg := imaging.Resize(img, w, h, imaging.Lanczos)
				if resizedImg == nil {
					return bo.handleImageError(bi, "resize", nil, kindle, cfg, log)
				}
				img = resizedImg
				bi.Dim.Width = img.Bounds().Dx()
				bi.Dim.Height = img.Bounds().Dy()
			}
		}

		if !cover && cfg.ScaleFactor > 0.0 && cfg.ScaleFactor != 1.0 {
			resizedImg := imaging.Resize(img, 0, int(float64(img.Bounds().Dy())*cfg.ScaleFactor), imaging.Linear)
			if resizedImg == nil {
				return bo.handleImageError(bi, "resize", nil, kindle, cfg, log)
			}
			img = resizedImg
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
		}

		data, encErr := bo.encodeImage(img, imgType, cfg, log)
		if encErr != nil {
			return bo.handleImageError(bi, "encode", encErr, kindle, cfg, log)
		}
		bi.Data = data
		return bi
	}

	imageChanged := false
	img, imgType, imgDecodingErr := image.Decode(bytes.NewReader(bo.Data))
	if imgDecodingErr != nil {
		return bo.handleImageError(bi, "decode", imgDecodingErr, kindle, cfg, log)
	}
	bi.MimeType = mime.TypeByExtension("." + imgType)
	bi.Dim.Width = img.Bounds().Dx()
	bi.Dim.Height = img.Bounds().Dy()

	// Scaling cover image
	if cover {
		w, h := cfg.Screen.Width, cfg.Screen.Height
		switch cfg.Cover.Resize {
		case common.ImageResizeModeNone:
		case common.ImageResizeModeKeepAR:
			if img.Bounds().Dy() >= h {
				break
			}
			resizedImg := imaging.Resize(img, 0, h, imaging.Lanczos)
			if resizedImg == nil {
				return bo.handleImageError(bi, "resize", nil, kindle, cfg, log)
			}
			img = resizedImg
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
			imageChanged = true
		case common.ImageResizeModeStretch:
			resizedImg := imaging.Resize(img, w, h, imaging.Lanczos)
			if resizedImg == nil {
				return bo.handleImageError(bi, "resize", nil, kindle, cfg, log)
			}
			img = resizedImg
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
			imageChanged = true
		}
	}

	// Scaling non-cover images
	if !cover && cfg.ScaleFactor > 0.0 && cfg.ScaleFactor != 1.0 {
		resizedImg := imaging.Resize(img, 0, int(float64(img.Bounds().Dy())*cfg.ScaleFactor), imaging.Linear)
		if resizedImg == nil {
			return bo.handleImageError(bi, "resize", nil, kindle, cfg, log)
		}
		img = resizedImg
		bi.Dim.Width = img.Bounds().Dx()
		bi.Dim.Height = img.Bounds().Dy()
		imageChanged = true
	}

	// Handle transparency for formats that support it (PNG, GIF)
	// JPEG doesn't support transparency, so we must flatten to white background
	if kindle || cfg.RemoveTransparency {
		if imgType == "png" || imgType == "gif" {
			opaque := func(im image.Image) bool {
				if oimg, ok := im.(interface{ Opaque() bool }); ok {
					return oimg.Opaque()
				}
				return true
			}(img)

			if !opaque {
				log.Debug("Removing image transparency", zap.String("id", bo.ID), zap.String("type", imgType))
				opaqueImg := image.NewRGBA(img.Bounds())
				draw.Draw(opaqueImg, img.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
				draw.Draw(opaqueImg, img.Bounds(), img, image.Point{}, draw.Over)
				img = opaqueImg
				imageChanged = true
				// GIF must be converted to PNG for re-encoding since encodeImage doesn't support GIF
				// (This will be overridden to JPEG below if kindle=true)
				if imgType == "gif" {
					imgType = "png"
					bi.MimeType = "image/png"
				}
			}
		}
	}

	// Compression & image quality
	if cfg.Optimize {
		switch imgType {
		case "jpeg":
			jr, err := jpegquality.NewFromBytes(bo.Data)
			if err != nil {
				log.Warn("Unable to detect JPEG quality level, skipping...", zap.String("id", bo.ID), zap.Error(err))
				break
			}

			q := jr.Quality()
			if q <= cfg.JPEGQuality {
				log.Debug("JPEG quality level already lower than requested, skipping...",
					zap.String("id", bo.ID), zap.Int("detected", q), zap.Int("requested", cfg.JPEGQuality))
				break
			}

			log.Debug("JPEG quality level higher than requested, reencoding...",
				zap.String("id", bo.ID), zap.Int("detected", q), zap.Int("requested", cfg.JPEGQuality))

			imageChanged = true
		case "png":
			imageChanged = true
		}
	}

	// Kindle compatibility: normalize any decodable raster format to JPEG.
	if kindle && imgType != "jpeg" {
		log.Debug("Converting image to jpeg for Kindle output",
			zap.String("id", bo.ID),
			zap.String("type", imgType))
		imgType = "jpeg"
		bi.MimeType = "image/jpeg"
		imageChanged = true
	}

	if !imageChanged {
		return bi
	}

	data, err := bo.encodeImage(img, imgType, cfg, log)
	if err != nil {
		return bo.handleImageError(bi, "encode", err, kindle, cfg, log)
	}
	bi.Data = data

	return bi
}

// isImageMIME returns true if the MIME type indicates an image resource
func isImageMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// mimeToExt returns file extension for common image MIME types
func mimeToExt(mimeType string) string {
	// Handle common types directly to prefer standard extensions
	switch strings.ToLower(mimeType) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/bmp":
		return "bmp"
	case "image/svg+xml":
		return "svg"
	case "image/webp":
		return "webp"
	case "image/tiff":
		return "tiff"
	}
	// Fallback to mime package for other types
	exts, err := mime.ExtensionsByType(mimeType)
	if err == nil && len(exts) > 0 {
		return strings.TrimPrefix(exts[0], ".")
	}
	return "img"
}
