package scanner

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/ivanlunardi/kdiff/differ"
)

// ScannedItem represents a single discovered file pair or single file between left and right.
type ScannedItem struct {
	RelativePath string
	LeftPath     string
	RightPath    string
	Status       differ.FileStatus
	IsBinary     bool
	LeftSize     int64
	RightSize    int64
}

// Options configures directory scanning behavior.
type Options struct {
	Full bool
}

type fileInfoEntry struct {
	fullPath string
	relPath  string
	size     int64
}

// Scan recursively analyzes leftDir and rightDir and produces a sorted list of ScannedItems.
func Scan(leftDir, rightDir string, opts Options) ([]ScannedItem, error) {
	leftFiles, err := collectFiles(leftDir, opts.Full)
	if err != nil {
		return nil, fmt.Errorf("scanning left directory %s: %w", leftDir, err)
	}

	rightFiles, err := collectFiles(rightDir, opts.Full)
	if err != nil {
		return nil, fmt.Errorf("scanning right directory %s: %w", rightDir, err)
	}

	allRelPathsMap := make(map[string]struct{}, len(leftFiles)+len(rightFiles))
	for rel := range leftFiles {
		allRelPathsMap[rel] = struct{}{}
	}
	for rel := range rightFiles {
		allRelPathsMap[rel] = struct{}{}
	}

	allRelPaths := slices.Collect(func(yield func(string) bool) {
		for rel := range allRelPathsMap {
			if !yield(rel) {
				return
			}
		}
	})
	slices.Sort(allRelPaths)

	items := make([]ScannedItem, 0, len(allRelPaths))
	for _, rel := range allRelPaths {
		leftEntry, hasLeft := leftFiles[rel]
		rightEntry, hasRight := rightFiles[rel]

		item := ScannedItem{
			RelativePath: rel,
		}

		if hasLeft {
			item.LeftPath = leftEntry.fullPath
			item.LeftSize = leftEntry.size
		}
		if hasRight {
			item.RightPath = rightEntry.fullPath
			item.RightSize = rightEntry.size
		}

		switch {
		case hasLeft && !hasRight:
			isBin, err := IsBinaryFile(leftEntry.fullPath)
			if err != nil {
				return nil, err
			}
			item.IsBinary = isBin
			item.Status = differ.StatusDeleted

		case !hasLeft && hasRight:
			isBin, err := IsBinaryFile(rightEntry.fullPath)
			if err != nil {
				return nil, err
			}
			item.IsBinary = isBin
			item.Status = differ.StatusAdded

		case hasLeft && hasRight:
			leftBin, err := IsBinaryFile(leftEntry.fullPath)
			if err != nil {
				return nil, err
			}
			rightBin, err := IsBinaryFile(rightEntry.fullPath)
			if err != nil {
				return nil, err
			}
			item.IsBinary = leftBin || rightBin

			sameContent, err := areFilesEqual(leftEntry.fullPath, rightEntry.fullPath, leftEntry.size, rightEntry.size)
			if err != nil {
				return nil, fmt.Errorf("comparing %s: %w", rel, err)
			}

			if sameContent {
				item.Status = differ.StatusIdentical
			} else {
				if item.IsBinary {
					item.Status = differ.StatusBinary
				} else {
					item.Status = differ.StatusModified
				}
			}
		}

		items = append(items, item)
	}

	return items, nil
}

func collectFiles(rootDir string, full bool) (map[string]fileInfoEntry, error) {
	files := make(map[string]fileInfoEntry)
	cleanRoot := filepath.Clean(rootDir)

	info, err := os.Stat(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", cleanRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", cleanRoot)
	}

	err = filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == cleanRoot {
			return nil
		}

		name := d.Name()
		if d.IsDir() {
			if ShouldExcludeDir(name, full) {
				return filepath.SkipDir
			}
			return nil
		}

		if ShouldExcludeFile(name, full) {
			return nil
		}

		rel, err := filepath.Rel(cleanRoot, path)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", path, err)
		}

		// Normalize to forward slashes
		rel = filepath.ToSlash(rel)

		fileInfo, err := d.Info()
		if err != nil {
			return fmt.Errorf("file info for %s: %w", path, err)
		}

		files[rel] = fileInfoEntry{
			fullPath: path,
			relPath:  rel,
			size:     fileInfo.Size(),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func areFilesEqual(leftPath, rightPath string, leftSize, rightSize int64) (bool, error) {
	if leftSize != rightSize {
		return false, nil
	}

	f1, err := os.Open(leftPath)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(rightPath)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	buf1 := make([]byte, 64*1024)
	buf2 := make([]byte, 64*1024)

	for {
		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false, nil
		}

		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return true, nil
			}
			if err1 == io.EOF || err2 == io.EOF {
				return false, nil
			}
			return false, cmp.Or(err1, err2)
		}
	}
}
