package fb2

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"mime"
	"strings"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"fbc/config"
	"fbc/jpegquality"
)

//go:embed broken.png
var brokenImagePNG []byte

// Image processing functions for FictionBook.

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

// PrepareImages processes all binary objects in the FictionBook creating
// actual image and building image index. Never returns an error - uses placeholder for broken images.
func (fb *FictionBook) PrepareImages(kindle bool, cfg *config.ImagesConfig, log *zap.Logger) BookImages {
	index := make(BookImages)

	imgNum := 1
	for i := range fb.Binaries {
		if _, exists := index[fb.Binaries[i].ID]; exists {
			log.Debug("Duplicate binary ID found, skipping", zap.String("id", fb.Binaries[i].ID))
			continue
		}
		cover := len(fb.Description.TitleInfo.Coverpage) > 0 && strings.HasSuffix(fb.Description.TitleInfo.Coverpage[0].Href, fb.Binaries[i].ID)
		bi := fb.Binaries[i].PrepareImage(kindle, cover, cfg, log)
		ext := mimeToExt(bi.MimeType)
		bi.Filename = fmt.Sprintf("img%05d.%s", imgNum, ext)
		imgNum++
		index[fb.Binaries[i].ID] = bi
	}
	return index
}

// isImageSupported returns true if image is supported and does not need
// conversion. Kindle devices support only GIF, BMP, JPEG and PNG formats.
func isImageSupported(format string) bool {
	imgType := strings.TrimPrefix(format, ".")
	for _, t := range [...]string{"gif", "bmp", "jpeg", "png"} {
		if strings.EqualFold(t, imgType) {
			return true
		}
	}
	return false
}

// JpegDPIType specifyes type of the DPI units
type jpegDPIType uint8

// DPI units type values
const (
	dpiNoUnits jpegDPIType = iota
	dpiPxPerInch
	dpiPxPerSm
)

// setJpegDPI creates JFIF APP0 with provided DPI if segment is missing in image.
// This is specific to go - when encoding jpeg standard encoder does not create
// JFIF APP0 segment and Kindles do not like it.
func setJpegDPI(buf *bytes.Buffer, dpit jpegDPIType, xdensity, ydensity int16) (*bytes.Buffer, bool) {

	var (
		marker = []byte{0xFF, 0xE0}                               // APP0 segment marker
		jfif   = []byte{0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x02} // jfif + version
	)

	data := buf.Bytes()

	// If JFIF APP0 segment is there - do not do anything
	if bytes.Equal(data[2:4], marker) {
		return buf, false
	}

	var newbuf = new(bytes.Buffer)

	newbuf.Write(data[:2])
	newbuf.Write(marker)
	binary.Write(newbuf, binary.BigEndian, uint16(0x10)) // length
	newbuf.Write(jfif)
	binary.Write(newbuf, binary.BigEndian, uint8(dpit))
	binary.Write(newbuf, binary.BigEndian, uint16(xdensity))
	binary.Write(newbuf, binary.BigEndian, uint16(ydensity))
	binary.Write(newbuf, binary.BigEndian, uint16(0)) // no thumbnail segment
	newbuf.Write(data[2:])

	return newbuf, true
}

// handleImageError is a unified error handler for all image processing failures.
// It logs the error and optionally substitutes the image with a placeholder.
func (bo *BinaryObject) handleImageError(bi *BookImage, operation string, err error, cfg *config.ImagesConfig, log *zap.Logger) *BookImage {
	// Log warning with appropriate context
	if err != nil {
		log.Warn("Unable to "+operation+" image", zap.String("id", bo.ID), zap.String("content-type", bo.ContentType), zap.Error(err))
	} else {
		log.Warn("Unable to "+operation+" image", zap.String("id", bo.ID), zap.String("content-type", bo.ContentType))
	}

	if !cfg.UseBroken {
		log.Debug("Substituting image with broken.png", zap.String("id", bo.ID))
		// Use embedded broken.png placeholder instead of broken data
		bi.Data = brokenImagePNG
		bi.MimeType = "image/png"
		// Decode the placeholder to get dimensions
		if img, _, decErr := image.Decode(bytes.NewReader(brokenImagePNG)); decErr == nil {
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
		}
	}
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
		err = imaging.Encode(buf, img, imaging.JPEG, imaging.JPEGQuality(cfg.JPEGQuality))
		if err != nil {
			return nil, fmt.Errorf("unable to encode processed JPEG, ID - %s: %w", bo.ID, err)
		}
		newbuf, added := setJpegDPI(buf, dpiPxPerInch, 300, 300)
		if added {
			log.Debug("Inserting jpeg JFIF APP0 marker segment", zap.String("id", bo.ID))
		}
		return newbuf.Bytes(), nil
	default:
		log.Warn("Unable to process image - unsupported format, skipping", zap.String("id", bo.ID), zap.String("type", imgType))
		return nil, nil
	}
}

// PrepareImage performs required image modifications leaving original data
// intact if no changes where requested. If image is decodable it will always
// attempt to normalize mime type. Never returns an error - uses placeholder for broken images.
func (bo *BinaryObject) PrepareImage(kindle, cover bool, cfg *config.ImagesConfig, log *zap.Logger) *BookImage {

	bi := &BookImage{
		MimeType: bo.ContentType,
		Data:     bo.Data,
	}

	// Special case - do not touch SVG
	if strings.HasSuffix(strings.ToLower(bo.ContentType), "svg") {
		bi.MimeType = "image/svg+xml"
		return bi
	}

	imageChanged := false
	img, imgType, imgDecodingErr := image.Decode(bytes.NewReader(bo.Data))
	if imgDecodingErr != nil {
		return bo.handleImageError(bi, "decode", imgDecodingErr, cfg, log)
	}
	bi.MimeType = mime.TypeByExtension("." + imgType)
	bi.Dim.Width = img.Bounds().Dx()
	bi.Dim.Height = img.Bounds().Dy()

	// Scaling cover image
	if cover {
		w, h := cfg.Cover.Width, cfg.Cover.Height
		switch cfg.Cover.Resize {
		case config.ImageResizeModeNone:
		case config.ImageResizeModeKeepAR:
			if img.Bounds().Dy() >= h {
				break
			}
			resizedImg := imaging.Resize(img, 0, h, imaging.Lanczos)
			if resizedImg == nil {
				return bo.handleImageError(bi, "resize", nil, cfg, log)
			}
			img = resizedImg
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
			imageChanged = true
		case config.ImageResizeModeStretch:
			resizedImg := imaging.Resize(img, w, h, imaging.Lanczos)
			if resizedImg == nil {
				return bo.handleImageError(bi, "resize", nil, cfg, log)
			}
			img = resizedImg
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
			imageChanged = true
		}
	}

	// Scaling non-cover images
	if !cover && cfg.ScaleFactor > 0.0 && cfg.ScaleFactor != 1.0 {
		if imgType == "png" || imgType == "jpeg" {
			resizedImg := imaging.Resize(img, 0, int(float64(img.Bounds().Dy())*cfg.ScaleFactor), imaging.Linear)
			if resizedImg == nil {
				return bo.handleImageError(bi, "resize", nil, cfg, log)
			}
			img = resizedImg
			bi.Dim.Width = img.Bounds().Dx()
			bi.Dim.Height = img.Bounds().Dy()
			imageChanged = true
		}
	}

	// PNG transparency
	if cfg.RemovePNGTransparency {
		if imgType == "png" {
			opaque := func(im image.Image) bool {
				if oimg, ok := im.(interface{ Opaque() bool }); ok {
					return oimg.Opaque()
				}
				return true
			}(img)

			if !opaque {
				log.Debug("Removing PNG transparency", zap.String("id", bo.ID))
				opaqueImg := image.NewRGBA(img.Bounds())
				draw.Draw(opaqueImg, img.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
				draw.Draw(opaqueImg, img.Bounds(), img, image.Point{}, draw.Over)
				img = opaqueImg
				imageChanged = true
			}
		}
	}

	// Compression & image quality
	if cfg.Optimize {
		switch imgType {
		case "jpeg":
			jr, err := jpegquality.NewWithBytes(bo.Data)
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

	// Kindle compatibility
	if kindle {
		if isImageSupported(imgType) && imgType != "jpeg" {
			log.Warn("Image type is not supported by target device, converting to jpeg",
				zap.String("id", bo.ID),
				zap.String("type", imgType))
			bi.MimeType = mime.TypeByExtension(".jpeg")
			imageChanged = true
		}
	}

	if !imageChanged {
		return bi
	}

	data, err := bo.encodeImage(img, imgType, cfg, log)
	if err != nil {
		return bo.handleImageError(bi, "encode", err, cfg, log)
	}
	if data != nil {
		bi.Data = data
	}

	return bi
}
