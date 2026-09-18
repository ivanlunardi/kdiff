package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// RunMetadata records summary information of a single comparison run.
type RunMetadata struct {
	ID            string    `json:"id"`
	DirectoryName string    `json:"directoryName"`
	Timestamp     time.Time `json:"timestamp"`
	FormattedDate string    `json:"formattedDate"`
	Title         string    `json:"title"`
	LeftPath      string    `json:"leftPath"`
	RightPath     string    `json:"rightPath"`
	TotalFiles    int       `json:"totalFiles"`
	AddedCount    int       `json:"addedCount"`
	DeletedCount  int       `json:"deletedCount"`
	ModifiedCount int       `json:"modifiedCount"`
	RelativeURL   string    `json:"relativeURL"`
}

type catalogData struct {
	Runs     []RunMetadata
	StyleCSS template.CSS
}

// LoadHistory loads all comparison runs from history.json in outputDir.
func LoadHistory(outputDir string) ([]RunMetadata, error) {
	historyFilePath := filepath.Join(outputDir, "history.json")

	data, err := os.ReadFile(historyFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunMetadata{}, nil
		}
		return nil, fmt.Errorf("reading history file %s: %w", historyFilePath, err)
	}

	var runs []RunMetadata
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, fmt.Errorf("parsing history metadata from %s: %w", historyFilePath, err)
	}

	// Ensure sorted by timestamp descending
	slices.SortFunc(runs, func(a, b RunMetadata) int {
		if a.Timestamp.After(b.Timestamp) {
			return -1
		}
		if a.Timestamp.Before(b.Timestamp) {
			return 1
		}
		return 0
	})

	return runs, nil
}

// UpdateHistoryCatalog adds a run to the history catalog and regenerates outputDir/index.html.
func UpdateHistoryCatalog(outputDir string, meta RunMetadata) error {
	historyFilePath := filepath.Join(outputDir, "history.json")

	var runs []RunMetadata
	if data, err := os.ReadFile(historyFilePath); err == nil {
		_ = json.Unmarshal(data, &runs)
	}

	// Avoid duplicate entries with the same ID
	runs = slices.DeleteFunc(runs, func(r RunMetadata) bool {
		return r.ID == meta.ID
	})

	runs = append([]RunMetadata{meta}, runs...)

	// Sort runs by timestamp descending
	slices.SortFunc(runs, func(a, b RunMetadata) int {
		if a.Timestamp.After(b.Timestamp) {
			return -1
		}
		if a.Timestamp.Before(b.Timestamp) {
			return 1
		}
		return 0
	})

	// Save history.json
	data, err := json.MarshalIndent(runs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling history metadata: %w", err)
	}
	if err := os.WriteFile(historyFilePath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", historyFilePath, err)
	}

	return RenderCatalog(outputDir, runs)
}

// RenderCatalog generates or refreshes the index.html catalog file in outputDir based on given runs.
func RenderCatalog(outputDir string, runs []RunMetadata) error {
	tmplContent, err := assetsFS.ReadFile("assets/history.html")
	if err != nil {
		return fmt.Errorf("reading history template: %w", err)
	}

	styleContent, err := assetsFS.ReadFile("assets/style.css")
	if err != nil {
		return fmt.Errorf("reading style asset: %w", err)
	}

	tmpl, err := template.New("history").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parsing history template: %w", err)
	}

	indexPath := filepath.Join(outputDir, "index.html")
	f, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", indexPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, catalogData{
		Runs:     runs,
		StyleCSS: template.CSS(styleContent),
	}); err != nil {
		return fmt.Errorf("executing history template: %w", err)
	}

	return nil
}

// EnsureCatalog verifies that index.html exists in outputDir, generating it if missing.
func EnsureCatalog(outputDir string) error {
	indexPath := filepath.Join(outputDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		return nil
	}
	runs, err := LoadHistory(outputDir)
	if err != nil {
		return err
	}
	return RenderCatalog(outputDir, runs)
}

// ClearHistory deletes all run_* directories, history.json, and index.html in outputDir.
// It returns the number of cleaned run directories.
func ClearHistory(outputDir string) (int, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading output directory %s: %w", outputDir, err)
	}

	cleanedCount := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "run_") {
			runPath := filepath.Join(outputDir, entry.Name())
			if err := os.RemoveAll(runPath); err != nil {
				return cleanedCount, fmt.Errorf("removing report directory %s: %w", runPath, err)
			}
			cleanedCount++
		}
	}

	historyFilePath := filepath.Join(outputDir, "history.json")
	if err := os.Remove(historyFilePath); err != nil && !os.IsNotExist(err) {
		return cleanedCount, fmt.Errorf("removing history file %s: %w", historyFilePath, err)
	}

	indexFilePath := filepath.Join(outputDir, "index.html")
	if err := os.Remove(indexFilePath); err != nil && !os.IsNotExist(err) {
		return cleanedCount, fmt.Errorf("removing catalog file %s: %w", indexFilePath, err)
	}

	return cleanedCount, nil
}
