package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ivanlunardi/kdiff/differ"
	"github.com/ivanlunardi/kdiff/scanner"
)

func TestScanner(t *testing.T) {
	tempDir := t.TempDir()
	leftDir := filepath.Join(tempDir, "left")
	rightDir := filepath.Join(tempDir, "right")

	mustMkdir(t, filepath.Join(leftDir, "sub", "node_modules"))
	mustMkdir(t, filepath.Join(rightDir, "sub", "node_modules"))
	mustMkdir(t, filepath.Join(leftDir, "sub", "src"))
	mustMkdir(t, filepath.Join(rightDir, "sub", "src"))

	// Identical file
	mustWriteFile(t, filepath.Join(leftDir, "sub", "src", "same.txt"), "hello world\n")
	mustWriteFile(t, filepath.Join(rightDir, "sub", "src", "same.txt"), "hello world\n")

	// Modified file
	mustWriteFile(t, filepath.Join(leftDir, "sub", "src", "mod.txt"), "version 1\n")
	mustWriteFile(t, filepath.Join(rightDir, "sub", "src", "mod.txt"), "version 2\n")

	// Deleted file (left only)
	mustWriteFile(t, filepath.Join(leftDir, "sub", "src", "deleted.txt"), "only on left\n")

	// Added file (right only)
	mustWriteFile(t, filepath.Join(rightDir, "sub", "src", "added.txt"), "only on right\n")

	// Binary modified file
	mustWriteFile(t, filepath.Join(leftDir, "sub", "src", "data.bin"), "bin\x00data1")
	mustWriteFile(t, filepath.Join(rightDir, "sub", "src", "data.bin"), "bin\x00data2")

	// File inside node_modules
	mustWriteFile(t, filepath.Join(leftDir, "sub", "node_modules", "pkg.json"), "{\"name\":\"pkg\"}")
	mustWriteFile(t, filepath.Join(rightDir, "sub", "node_modules", "pkg.json"), "{\"name\":\"pkg2\"}")

	t.Run("Default exclusion skips node_modules", func(t *testing.T) {
		items, err := scanner.Scan(leftDir, rightDir, scanner.Options{Full: false})
		if err != nil {
			t.Fatalf("unexpected scan error: %v", err)
		}

		itemMap := make(map[string]scanner.ScannedItem)
		for _, item := range items {
			itemMap[item.RelativePath] = item
		}

		if _, exists := itemMap["sub/node_modules/pkg.json"]; exists {
			t.Errorf("expected sub/node_modules/pkg.json to be excluded, but was present")
		}

		if item, exists := itemMap["sub/src/same.txt"]; !exists || item.Status != differ.StatusIdentical {
			t.Errorf("expected same.txt to be IDENTICAL, got %v", item.Status)
		}

		if item, exists := itemMap["sub/src/mod.txt"]; !exists || item.Status != differ.StatusModified {
			t.Errorf("expected mod.txt to be MODIFIED, got %v", item.Status)
		}

		if item, exists := itemMap["sub/src/deleted.txt"]; !exists || item.Status != differ.StatusDeleted {
			t.Errorf("expected deleted.txt to be DELETED, got %v", item.Status)
		}

		if item, exists := itemMap["sub/src/added.txt"]; !exists || item.Status != differ.StatusAdded {
			t.Errorf("expected added.txt to be ADDED, got %v", item.Status)
		}

		if item, exists := itemMap["sub/src/data.bin"]; !exists || item.Status != differ.StatusBinary || !item.IsBinary {
			t.Errorf("expected data.bin to be BINARY and IsBinary=true, got status=%v isBinary=%v", item.Status, item.IsBinary)
		}
	})

	t.Run("Full flag includes node_modules", func(t *testing.T) {
		items, err := scanner.Scan(leftDir, rightDir, scanner.Options{Full: true})
		if err != nil {
			t.Fatalf("unexpected scan error: %v", err)
		}

		itemMap := make(map[string]scanner.ScannedItem)
		for _, item := range items {
			itemMap[item.RelativePath] = item
		}

		if item, exists := itemMap["sub/node_modules/pkg.json"]; !exists || item.Status != differ.StatusModified {
			t.Errorf("expected sub/node_modules/pkg.json to be included and MODIFIED with Full=true, got %v (exists=%v)", item.Status, exists)
		}
	})
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
