package fb2

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/h2non/filetype"
	"go.uber.org/zap"
)

var (
	// Match url() references: url("path") or url('path') or url(path)
	urlPattern = regexp.MustCompile(`url\s*\(\s*['"]?([^'")]+)['"]?\s*\)`)

	// Match @font-face blocks
	fontFacePattern = regexp.MustCompile(`@font-face\s*\{[^}]*\}`)

	// Match @import statements
	importPattern = regexp.MustCompile(`@import\s+(?:url\s*\()?\s*['"]?([^'"()]+)['"]?\s*\)?`)
)

// cssExternalRef represents a reference found in CSS
type cssExternalRef struct {
	URL     string // URL as it appears in CSS
	Context string // "font-face", "import", "background", etc.
}

// NormalizeStylesheets processes all stylesheets (including optional default stylesheet)
// and resolves external resource references against the book's binary objects or loads
// them from external files.
// This should be called after binaries are loaded but before generating output.
// srcPath is the path to the source FB2 file, used to resolve relative paths.
// defaultCSS is optional default stylesheet data to prepend (can be nil or empty).
// Modifies the FictionBook in place.
func (fb *FictionBook) NormalizeStylesheets(srcPath string, defaultCSS []byte, log *zap.Logger) *FictionBook {
	// Prepend default stylesheet if provided
	if len(defaultCSS) > 0 {
		defaultStylesheet := Stylesheet{
			Type: "text/css",
			Data: string(defaultCSS),
		}
		fb.Stylesheets = append([]Stylesheet{defaultStylesheet}, fb.Stylesheets...)
		log.Debug("Added default stylesheet for processing", zap.Int("bytes", len(defaultCSS)))
	}

	// Early return if no stylesheets to process
	if len(fb.Stylesheets) == 0 {
		return fb
	}

	// Build binary ID lookup
	binaryIndex := make(map[string]*BinaryObject)
	for i := range fb.Binaries {
		binaryIndex[fb.Binaries[i].ID] = &fb.Binaries[i]
	}

	// Get source directory for resolving relative paths in FB2 embedded stylesheets
	srcDir := filepath.Dir(srcPath)

	for i := range fb.Stylesheets {
		sheet := &fb.Stylesheets[i]

		if sheet.Type != "text/css" {
			log.Debug("Skipping non-CSS stylesheet", zap.String("type", sheet.Type))
			continue
		}

		refs := parseStylesheetResources(sheet.Data)

		if len(refs) == 0 {
			continue
		}

		log.Debug("Found external references in stylesheet", zap.Int("count", len(refs)))

		// Determine base path for resource resolution
		// First stylesheet is the default CSS (if provided) - resolve relative to current directory
		// Remaining stylesheets are FB2 embedded - resolve relative to source file directory
		basePath := srcDir
		if i == 0 && len(defaultCSS) > 0 {
			// Default stylesheet - use current working directory
			basePath = "."
			log.Debug("Resolving default stylesheet resources from current directory")
		}

		for _, ref := range refs {
			// Skip data: URLs (already embedded)
			if strings.HasPrefix(ref.URL, "data:") {
				log.Debug("Skipping data URL", zap.String("url", ref.URL[:min(50, len(ref.URL))]))
				continue
			}

			// Warn about absolute HTTP(S) URLs - cannot be embedded
			if strings.HasPrefix(ref.URL, "http://") || strings.HasPrefix(ref.URL, "https://") {
				log.Warn("External URL in stylesheet cannot be embedded in EPUB",
					zap.String("url", ref.URL),
					zap.String("context", ref.Context))
				continue
			}

			// Try to resolve reference
			resource := fb.resolveStylesheetResource(ref, binaryIndex, basePath, log)
			if resource != nil {
				sheet.Resources = append(sheet.Resources, *resource)
			}
		}
	}

	return fb
}

// resolveStylesheetResource attempts to resolve a CSS resource reference
// First tries to find it in binaries, then attempts to load from filesystem
// basePath is the directory to use for resolving relative URLs (either source directory or current directory)
func (fb *FictionBook) resolveStylesheetResource(ref cssExternalRef, binaryIndex map[string]*BinaryObject, basePath string, log *zap.Logger) *StylesheetResource {
	// Case 1: Fragment reference (#id) - look in binaries
	if resourceID, isFragment := strings.CutPrefix(ref.URL, "#"); isFragment {
		binary, found := binaryIndex[resourceID]
		if !found {
			log.Warn("Stylesheet references non-existent binary",
				zap.String("url", ref.URL),
				zap.String("id", resourceID),
				zap.String("context", ref.Context))
			return nil
		}

		// Use only basename for filename
		baseName := filepath.Base(resourceID)

		// Determine directory based on content type
		var dir string
		if isFontMIME(binary.ContentType) {
			dir = FontsDir
		} else {
			dir = OtherDir
		}

		resource := &StylesheetResource{
			OriginalURL: ref.URL,
			ResolvedID:  resourceID,
			MimeType:    binary.ContentType,
			Data:        binary.Data,
			Filename:    baseName,
			Loaded:      false,
		}

		// Ensure filename has extension
		if filepath.Ext(resource.Filename) == "" {
			ext := mimeToExtension(binary.ContentType)
			resource.Filename = resource.Filename + ext
		}

		// Set full path with directory
		resource.Filename = filepath.Join(dir, resource.Filename)

		log.Debug("Resolved stylesheet resource from binary",
			zap.String("url", ref.URL),
			zap.String("id", resourceID),
			zap.String("filename", resource.Filename),
			zap.String("mime", resource.MimeType),
			zap.Int("bytes", len(resource.Data)))

		return resource
	}

	// Case 2: File path (relative or absolute) - load from filesystem
	resourcePath := ref.URL

	// Handle relative paths
	if !filepath.IsAbs(resourcePath) {
		resourcePath = filepath.Join(basePath, resourcePath)
	}

	// Clean the path (remove ../ etc.)
	resourcePath = filepath.Clean(resourcePath)

	// Try to load the file
	data, err := os.ReadFile(resourcePath)
	if err != nil {
		log.Warn("Unable to load stylesheet resource from file",
			zap.String("url", ref.URL),
			zap.String("path", resourcePath),
			zap.String("context", ref.Context),
			zap.Error(err))
		return nil
	}

	// Detect MIME type - prefer extension-based detection for fonts
	mimeType := ""
	if ext := filepath.Ext(ref.URL); ext != "" {
		mimeType = extToMimeType(ext)
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	if !validateLoadedResource(mimeType, data) {
		log.Warn("Loaded stylesheet resource failed validation",
			zap.String("url", ref.URL),
			zap.String("path", resourcePath),
			zap.String("context", ref.Context))
		return nil
	}

	// Use only basename from original URL for the filename
	baseName := filepath.Base(ref.URL)

	// Generate unique ID for this resource (without extension)
	resourceID := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	resourceID = sanitizeResourceFilename(resourceID)
	if resourceID == "" || resourceID == "." {
		resourceID = fmt.Sprintf("loaded-resource-%d", len(fb.Binaries))
	}

	// Make sure ID is unique
	counter := 0
	originalID := resourceID
	for {
		if _, exists := binaryIndex[resourceID]; !exists {
			break
		}
		counter++
		resourceID = fmt.Sprintf("%s-%d", originalID, counter)
	}

	// Add to binaries
	fb.Binaries = append(fb.Binaries, BinaryObject{
		ID:          resourceID,
		ContentType: mimeType,
		Data:        data,
	})
	// Update index
	binaryIndex[resourceID] = &fb.Binaries[len(fb.Binaries)-1]

	// Determine directory based on MIME type
	var dir string
	if isFontMIME(mimeType) {
		dir = FontsDir
	} else {
		dir = OtherDir
	}

	resource := &StylesheetResource{
		OriginalURL: ref.URL,
		ResolvedID:  resourceID,
		MimeType:    mimeType,
		Data:        data,
		Filename:    baseName,
		Loaded:      true,
	}

	// Ensure filename has extension
	if filepath.Ext(resource.Filename) == "" {
		ext := mimeToExtension(mimeType)
		resource.Filename = resource.Filename + ext
	}

	// Set full path with directory
	resource.Filename = filepath.Join(dir, resource.Filename)

	log.Info("Loaded stylesheet resource from file",
		zap.String("url", ref.URL),
		zap.String("path", resourcePath),
		zap.String("id", resourceID),
		zap.String("filename", resource.Filename),
		zap.String("mime", mimeType),
		zap.Int("bytes", len(data)))

	return resource
}

// parseStylesheetResources extracts external resource references from CSS
func parseStylesheetResources(css string) []cssExternalRef {
	var refs []cssExternalRef
	seen := make(map[string]bool)

	// Find @import statements
	for _, match := range importPattern.FindAllStringSubmatch(css, -1) {
		url := strings.TrimSpace(match[1])
		if url != "" && !seen[url] {
			refs = append(refs, cssExternalRef{
				URL:     url,
				Context: "import",
			})
			seen[url] = true
		}
	}

	// Find @font-face blocks and their url() references
	for _, fontFaceBlock := range fontFacePattern.FindAllString(css, -1) {
		for _, match := range urlPattern.FindAllStringSubmatch(fontFaceBlock, -1) {
			url := strings.TrimSpace(match[1])
			if url != "" && !seen[url] {
				refs = append(refs, cssExternalRef{
					URL:     url,
					Context: "font-face",
				})
				seen[url] = true
			}
		}
	}

	// Find other url() references (backgrounds, borders, etc.)
	// Skip those already found in @font-face or @import
	allURLs := urlPattern.FindAllStringSubmatch(css, -1)
	for _, match := range allURLs {
		url := strings.TrimSpace(match[1])
		if url != "" && !seen[url] {
			refs = append(refs, cssExternalRef{
				URL:     url,
				Context: "other",
			})
			seen[url] = true
		}
	}

	return refs
}

// sanitizeResourceFilename creates a safe filename from URL
func sanitizeResourceFilename(url string) string {
	// Remove fragment identifier
	url = strings.TrimPrefix(url, "#")

	// Remove path traversal attempts
	url = strings.ReplaceAll(url, "..", "")
	url = strings.TrimPrefix(url, "/")

	// Get base filename
	base := filepath.Base(url)

	// If no extension or empty, return as-is
	if base == "" || base == "." {
		return ""
	}

	return base
}

// validateLoadedResource performs additional sanity checks on loaded resource data
func validateLoadedResource(mimeType string, data []byte) bool {
	// do additional sanity check
	switch mimeType {
	case "font/woff":
		return filetype.Is(data, "woff")
	case "font/woff2":
		return filetype.Is(data, "woff2")
	case "font/ttf":
		return filetype.Is(data, "ttf")
	case "font/otf":
		return filetype.Is(data, "otf")
	}
	return true
}

// mimeToExtension returns file extension for common MIME types
func mimeToExtension(mimeType string) string {
	switch mimeType {
	case "font/woff":
		return ".woff"
	case "font/woff2":
		return ".woff2"
	case "font/ttf", "application/x-font-ttf", "application/font-sfnt":
		return ".ttf"
	case "font/otf", "application/x-font-otf":
		return ".otf"
	case "application/font-woff":
		return ".woff"
	case "application/font-woff2":
		return ".woff2"
	case "image/svg+xml":
		return ".svg"
	case "application/vnd.ms-fontobject":
		return ".eot"
	default:
		return ""
	}
}

// extToMimeType returns MIME type for common font file extensions
func extToMimeType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".svg":
		return "image/svg+xml"
	default:
		return ""
	}
}

// isFontMIME returns true if the MIME type indicates a font resource
func isFontMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "font/") ||
		strings.HasPrefix(mimeType, "application/font-") ||
		strings.HasPrefix(mimeType, "application/x-font-") ||
		mimeType == "application/vnd.ms-fontobject"
}
