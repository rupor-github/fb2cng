package fb2

import (
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/h2non/filetype"
	"go.uber.org/zap"

	"fbc/css"
)

// urlExtractPattern extracts URLs from raw CSS value strings such as @font-face src
// or background property values. It matches url("path"), url('path'), and url(path).
var urlExtractPattern = regexp.MustCompile(`url\s*\(\s*(?:["']([^"']*)["']|([^)"]*))\s*\)`)

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

	// Extract section page-break information from all stylesheets.
	// Process in order so that later stylesheets (user CSS) override earlier ones (default CSS).
	fb.sectionPageBreaks = make(map[int]bool)
	for i := range fb.Stylesheets {
		if fb.Stylesheets[i].Type != "text/css" {
			continue
		}
		maps.Copy(fb.sectionPageBreaks, parseSectionPageBreaks(fb.Stylesheets[i].Data))
	}

	if len(fb.sectionPageBreaks) > 0 {
		log.Debug("Extracted section page breaks from CSS",
			zap.Any("breaks", fb.sectionPageBreaks))
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

	// Case 2: File path — load from filesystem using os.DirFS for security.
	// os.DirFS roots a filesystem at basePath and refuses to serve absolute
	// paths or paths containing ".." that would escape the root. This
	// prevents path-traversal attacks (e.g. url('../../etc/passwd')).
	resourcePath := ref.URL

	// os.DirFS uses forward-slash paths (fs.FS convention), so normalize.
	resourcePath = filepath.ToSlash(resourcePath)

	baseFS := os.DirFS(basePath)
	data, err := fs.ReadFile(baseFS, resourcePath)
	if err != nil {
		log.Warn("Unable to load stylesheet resource from file",
			zap.String("url", ref.URL),
			zap.String("basePath", basePath),
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
	resource.Filename = path.Join(dir, resource.Filename)

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
// by parsing it with the css package and walking the AST.
func parseStylesheetResources(cssText string) []cssExternalRef {
	sheet := css.NewParser(nil).Parse([]byte(cssText))

	var refs []cssExternalRef
	seen := make(map[string]bool)

	addURL := func(url, context string) {
		url = strings.TrimSpace(url)
		if url != "" && !seen[url] {
			refs = append(refs, cssExternalRef{URL: url, Context: context})
			seen[url] = true
		}
	}

	// extractURLs pulls url() references out of a raw CSS value string.
	extractURLs := func(raw, context string) {
		for _, m := range urlExtractPattern.FindAllStringSubmatch(raw, -1) {
			// Group 1 is quoted URL, group 2 is unquoted
			u := m[1]
			if u == "" {
				u = m[2]
			}
			addURL(u, context)
		}
	}

	for _, item := range sheet.Items {
		switch {
		case item.Import != nil:
			addURL(*item.Import, "import")

		case item.FontFace != nil:
			extractURLs(item.FontFace.Src, "font-face")

		case item.Rule != nil:
			for _, val := range item.Rule.Properties {
				if strings.Contains(val.Raw, "url(") {
					extractURLs(val.Raw, "other")
				}
			}

		case item.MediaBlock != nil:
			for _, rule := range item.MediaBlock.Rules {
				for _, val := range rule.Properties {
					if strings.Contains(val.Raw, "url(") {
						extractURLs(val.Raw, "other")
					}
				}
			}
		}
	}

	return refs
}

// parseSectionPageBreaks scans CSS text for .section-title-hN rules that
// contain page-break-before:always and returns a map of depth -> bool.
// Later stylesheets should be processed after earlier ones so that user
// CSS can override defaults — the caller is responsible for merging.
func parseSectionPageBreaks(cssText string) map[int]bool {
	sheet := css.NewParser(nil).Parse([]byte(cssText))
	breaks := make(map[int]bool)

	// checkRule examines a single rule for .section-title-hN selectors
	// and page-break-before: always property.
	checkRule := func(rule *css.Rule) {
		class := rule.Selector.Class
		if !strings.HasPrefix(class, "section-title-h") {
			return
		}
		depthStr := strings.TrimPrefix(class, "section-title-h")
		depth, err := strconv.Atoi(depthStr)
		if err != nil || depth < 2 || depth > 6 {
			return
		}
		// Check if the rule has page-break-before: always
		val, ok := rule.GetProperty("page-break-before")
		breaks[depth] = ok && strings.EqualFold(val.Raw, "always")
	}

	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			checkRule(item.Rule)
		case item.MediaBlock != nil:
			for i := range item.MediaBlock.Rules {
				checkRule(&item.MediaBlock.Rules[i])
			}
		}
	}

	return breaks
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
