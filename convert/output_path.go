package convert

import (
	_ "embed"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gosimple/slug"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/state"
)

// buildOutputPath returns constructed output file path/name based on various
// input parameters. It uses either default naming scheme or user-defined
// template and takes into account whether to preserve source directory
// structure on the output. If cleans up path and if requested transliterates
// it
func buildOutputPath(c *content.Content, src, dst string, env *state.LocalEnv) string {
	outDir := determineOutputDir(src, dst, env)
	defaultFile := buildDefaultFileName(src, c.OutputFormat, env)

	if env.Cfg.Document.OutputNameTemplate == "" {
		return filepath.Join(outDir, defaultFile)
	}

	expandedName := expandOutputNameTemplate(c, env)
	if expandedName == "" {
		// fallback to default name if template expansion failed
		return filepath.Join(outDir, defaultFile)
	}

	return assemblePathWithSubdirs(outDir, expandedName, c.OutputFormat, env)
}

func determineOutputDir(src, dst string, env *state.LocalEnv) string {
	if env.NoDirs {
		return dst
	}
	return filepath.Join(dst, filepath.Dir(src))
}

func buildDefaultFileName(src string, format config.OutputFmt, env *state.LocalEnv) string {
	baseName := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	if env.Cfg.Document.FileNameTransliterate {
		baseName = slug.Make(baseName)
	}
	return config.CleanFileName(baseName) + getFileExtension(format)
}

func getFileExtension(format config.OutputFmt) string {
	switch format {
	case config.OutputFmtKfx:
		return ".kfx"
	case config.OutputFmtEpub2, config.OutputFmtEpub3:
		return ".epub"
	case config.OutputFmtKepub:
		return ".kepub.epub"
	default:
		// this should never happen
		panic("unsupported format requested")
	}
}

func expandOutputNameTemplate(c *content.Content, env *state.LocalEnv) string {
	expandedName, err := expandTemplate(c, config.OutputNameTemplateFieldName, env.Cfg.Document.OutputNameTemplate, c.OutputFormat)
	if err != nil {
		env.Log.Warn("Unable to prepare output filename", zap.Error(err))
		return ""
	}
	return filepath.FromSlash(expandedName)
}

// assemblePathWithSubdirs takes an expanded template name (which may contain
// path separators for subdirectories) and assembles it into a full output path,
// cleaning and transliterating segments as needed
func assemblePathWithSubdirs(outDir, expandedName string, format config.OutputFmt, env *state.LocalEnv) string {
	outExt := getFileExtension(format)
	pathSegments := splitAndCleanPath(expandedName)

	if len(pathSegments) == 0 {
		return outDir
	}

	fileName := cleanPathSegment(pathSegments[len(pathSegments)-1], env) + outExt
	dirParts := make([]string, 0, len(pathSegments)+1)
	dirParts = append(dirParts, outDir)

	for _, segment := range pathSegments[:len(pathSegments)-1] {
		dirParts = append(dirParts, cleanPathSegment(segment, env))
	}

	dirParts = append(dirParts, fileName)
	return filepath.Join(dirParts...)
}

func splitAndCleanPath(path string) []string {
	path = strings.TrimSuffix(path, string(os.PathSeparator))
	segments := make([]string, 0, 8)

	for head, tail := filepath.Split(path); tail != ""; head, tail = filepath.Split(head) {
		segments = slices.Insert(segments, 0, tail)
		head = strings.TrimSuffix(head, string(os.PathSeparator))
		if head == "" {
			break
		}
		path = head
	}

	return segments
}

func cleanPathSegment(segment string, env *state.LocalEnv) string {
	if env.Cfg.Document.FileNameTransliterate {
		segment = slug.Make(segment)
	}
	return config.CleanFileName(segment)
}
