package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ivanlunardi/kdiff/differ"
	"github.com/ivanlunardi/kdiff/report"
)

func TestReportGeneration(t *testing.T) {
	tempDir := t.TempDir()

	comp1 := differ.ComparisonReport{
		ID:             "test-run-1",
		Timestamp:      time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
		Title:          "First Comparison",
		LeftPath:       "/path/to/left",
		RightPath:      "/path/to/right",
		TotalFiles:     3,
		AddedCount:     1,
		DeletedCount:   1,
		ModifiedCount:  1,
		IdenticalCount: 0,
		Files: []differ.FileDiff{
			{
				RelativePath: "main.go",
				Status:       differ.StatusModified,
				IsBinary:     false,
				LeftSize:     100,
				RightSize:    120,
				Additions:    2,
				Deletions:    1,
				Lines: []differ.DiffLine{
					{LeftLineNum: 1, RightLineNum: 1, Type: differ.LineEqual, Content: "package main"},
					{LeftLineNum: 2, RightLineNum: 0, Type: differ.LineDelete, Content: "old code"},
					{LeftLineNum: 0, RightLineNum: 2, Type: differ.LineInsert, Content: "new code"},
				},
			},
			{
				RelativePath: "added.go",
				Status:       differ.StatusAdded,
				Additions:    1,
				Lines: []differ.DiffLine{
					{LeftLineNum: 0, RightLineNum: 1, Type: differ.LineInsert, Content: "package added"},
				},
			},
			{
				RelativePath: "deleted.go",
				Status:       differ.StatusDeleted,
				Deletions:    1,
				Lines: []differ.DiffLine{
					{LeftLineNum: 1, RightLineNum: 0, Type: differ.LineDelete, Content: "package deleted"},
				},
			},
		},
	}

	indexPath1, err := report.Generate(comp1, tempDir)
	if err != nil {
		t.Fatalf("failed to generate first report: %v", err)
	}

	// Verify run index.html
	if _, err := os.Stat(indexPath1); err != nil {
		t.Fatalf("expected run index.html at %s, got error: %v", indexPath1, err)
	}
	content1, err := os.ReadFile(indexPath1)
	if err != nil {
		t.Fatalf("failed to read run index.html: %v", err)
	}
	if !strings.Contains(string(content1), "First Comparison") {
		t.Errorf("expected report content to contain Title 'First Comparison'")
	}
	if !strings.Contains(string(content1), "main.go") {
		t.Errorf("expected report content to contain 'main.go'")
	}
	if !strings.Contains(string(content1), "btn-prev-file") || !strings.Contains(string(content1), "btn-next-file") {
		t.Errorf("expected report content to contain navigation buttons 'btn-prev-file' and 'btn-next-file'")
	}
	if !strings.Contains(string(content1), "file-counter") {
		t.Errorf("expected report content to contain 'file-counter'")
	}
	if !strings.Contains(string(content1), "navigateFile") || !strings.Contains(string(content1), "ArrowDown") {
		t.Errorf("expected report script to contain arrow keyboard navigation logic")
	}
	if !strings.Contains(string(content1), "file-nav-group") {
		t.Errorf("expected report style to contain 'file-nav-group'")
	}

	// Verify catalog index.html
	catalogPath := filepath.Join(tempDir, "index.html")
	if _, err := os.Stat(catalogPath); err != nil {
		t.Fatalf("expected catalog index.html at %s, got error: %v", catalogPath, err)
	}
	catalogContent, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("failed to read catalog index.html: %v", err)
	}
	if !strings.Contains(string(catalogContent), "First Comparison") {
		t.Errorf("expected catalog content to contain 'First Comparison'")
	}

	// Run second report
	comp2 := differ.ComparisonReport{
		ID:            "test-run-2",
		Timestamp:     time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC),
		Title:         "Second Comparison",
		LeftPath:      "/path/to/left",
		RightPath:     "/path/to/right",
		TotalFiles:    1,
		ModifiedCount: 1,
		Files:         []differ.FileDiff{},
	}

	indexPath2, err := report.Generate(comp2, tempDir)
	if err != nil {
		t.Fatalf("failed to generate second report: %v", err)
	}

	if _, err := os.Stat(indexPath2); err != nil {
		t.Fatalf("expected second run index.html at %s, got error: %v", indexPath2, err)
	}

	// Check catalog now has both runs
	catalogContent2, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("failed to read updated catalog: %v", err)
	}
	if !strings.Contains(string(catalogContent2), "First Comparison") || !strings.Contains(string(catalogContent2), "Second Comparison") {
		t.Errorf("expected catalog to contain both runs")
	}
	if !strings.Contains(string(catalogContent2), "paths-cell") || !strings.Contains(string(catalogContent2), "action-cell") {
		t.Errorf("expected catalog to contain 'paths-cell' and 'action-cell' classes")
	}
}

func TestLoadHistory(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Empty directory
	runs, err := report.LoadHistory(tempDir)
	if err != nil {
		t.Fatalf("unexpected error loading from empty dir: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected 0 runs, got %d", len(runs))
	}

	// 2. Generate a run
	comp := differ.ComparisonReport{
		ID:        "run-load-test",
		Timestamp: time.Now(),
		Title:     "Load History Test Run",
		LeftPath:  "/left/very/long/path/to/source/folder",
		RightPath: "/right/very/long/path/to/destination/folder",
		Files:     []differ.FileDiff{},
	}
	if _, err := report.Generate(comp, tempDir); err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	loadedRuns, err := report.LoadHistory(tempDir)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	if len(loadedRuns) != 1 {
		t.Fatalf("expected 1 run, got %d", len(loadedRuns))
	}
	if loadedRuns[0].Title != "Load History Test Run" {
		t.Errorf("expected title 'Load History Test Run', got %s", loadedRuns[0].Title)
	}
}

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "Hello_World"},
		{"v1.0.0 (Release)", "v100_Release"},
		{"  spaces  ", "spaces"},
		{"", ""},
		{"feature/branch#123", "featurebranch123"},
	}

	for _, tt := range tests {
		got := report.SanitizeTitle(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
