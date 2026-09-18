package report_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ivanlunardi/kdiff/differ"
	"github.com/ivanlunardi/kdiff/report"
)

func TestClearHistory(t *testing.T) {
	t.Run("Non-existent directory returns 0 and no error", func(t *testing.T) {
		tempDir := filepath.Join(t.TempDir(), "does_not_exist")
		count, err := report.ClearHistory(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0, got %d", count)
		}
	})

	t.Run("Empty directory returns 0 and no error", func(t *testing.T) {
		tempDir := t.TempDir()
		count, err := report.ClearHistory(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected count 0, got %d", count)
		}
	})

	t.Run("Cleans run directories, history.json, index.html, and keeps unrelated files", func(t *testing.T) {
		tempDir := t.TempDir()

		// Generate 2 reports
		comp1 := differ.ComparisonReport{
			ID:        "run-1",
			Timestamp: time.Now(),
			Title:     "Run 1",
			LeftPath:  "/left1",
			RightPath: "/right1",
			Files:     []differ.FileDiff{},
		}
		if _, err := report.Generate(comp1, tempDir); err != nil {
			t.Fatalf("failed to generate report 1: %v", err)
		}

		comp2 := differ.ComparisonReport{
			ID:        "run-2",
			Timestamp: time.Now(),
			Title:     "Run 2",
			LeftPath:  "/left2",
			RightPath: "/right2",
			Files:     []differ.FileDiff{},
		}
		if _, err := report.Generate(comp2, tempDir); err != nil {
			t.Fatalf("failed to generate report 2: %v", err)
		}

		// Create an unrelated file and directory
		unrelatedFile := filepath.Join(tempDir, "keep_me.txt")
		if err := os.WriteFile(unrelatedFile, []byte("important user file"), 0644); err != nil {
			t.Fatalf("failed to write unrelated file: %v", err)
		}
		unrelatedDir := filepath.Join(tempDir, "custom_folder")
		if err := os.MkdirAll(unrelatedDir, 0755); err != nil {
			t.Fatalf("failed to create unrelated directory: %v", err)
		}

		// Clear history
		count, err := report.ClearHistory(tempDir)
		if err != nil {
			t.Fatalf("unexpected error during ClearHistory: %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}

		// Verify history.json is gone
		if _, err := os.Stat(filepath.Join(tempDir, "history.json")); !os.IsNotExist(err) {
			t.Errorf("expected history.json to be deleted")
		}

		// Verify index.html is gone
		if _, err := os.Stat(filepath.Join(tempDir, "index.html")); !os.IsNotExist(err) {
			t.Errorf("expected index.html to be deleted")
		}

		// Verify run_* folders are gone
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("failed to read tempDir: %v", err)
		}
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "custom_folder" {
				t.Errorf("unexpected directory remained: %s", entry.Name())
			}
		}

		// Verify unrelated file and directory remain
		if _, err := os.Stat(unrelatedFile); err != nil {
			t.Errorf("expected unrelated file to remain, err: %v", err)
		}
		if _, err := os.Stat(unrelatedDir); err != nil {
			t.Errorf("expected unrelated dir to remain, err: %v", err)
		}
	})
}

func TestEnsureCatalog(t *testing.T) {
	t.Run("Generates index.html when missing", func(t *testing.T) {
		tempDir := t.TempDir()
		indexPath := filepath.Join(tempDir, "index.html")

		if err := report.EnsureCatalog(tempDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("failed to read created index.html: %v", err)
		}
		if len(data) == 0 {
			t.Errorf("expected non-empty index.html")
		}
	})

	t.Run("Does not overwrite existing index.html", func(t *testing.T) {
		tempDir := t.TempDir()
		indexPath := filepath.Join(tempDir, "index.html")
		customContent := []byte("custom content")
		if err := os.WriteFile(indexPath, customContent, 0644); err != nil {
			t.Fatalf("failed to write custom index.html: %v", err)
		}

		if err := report.EnsureCatalog(tempDir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatalf("failed to read index.html: %v", err)
		}
		if string(data) != "custom content" {
			t.Errorf("expected index.html to retain original content, got %s", string(data))
		}
	})
}
