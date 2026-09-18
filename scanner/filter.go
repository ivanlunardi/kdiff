package scanner

import (
	"strings"
)

var defaultExcludedDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	".svn":         true,
	".hg":          true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".cache":       true,
	"__pycache__":  true,
	".next":        true,
	".nuxt":        true,
	".idea":        true,
	".vscode":      true,
	"bin":          true,
	"obj":          true,
}

var defaultExcludedFiles = map[string]bool{
	".DS_Store": true,
	"Thumbs.db": true,
}

// ShouldExcludeDir returns true if the directory name should be excluded.
func ShouldExcludeDir(dirName string, full bool) bool {
	if full {
		return false
	}
	base := strings.TrimSpace(dirName)
	return defaultExcludedDirs[base]
}

// ShouldExcludeFile returns true if the file should be excluded.
func ShouldExcludeFile(fileName string, full bool) bool {
	if full {
		return false
	}
	base := strings.TrimSpace(fileName)
	return defaultExcludedFiles[base]
}
