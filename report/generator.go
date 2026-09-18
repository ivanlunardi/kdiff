package report

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ivanlunardi/kdiff/differ"
)

//go:embed assets/*
var assetsFS embed.FS

type reportTemplateData struct {
	Title          string
	LeftPath       string
	RightPath      string
	Timestamp      string
	TotalFiles     int
	AddedCount     int
	DeletedCount   int
	ModifiedCount  int
	IdenticalCount int
	ReportJSON     template.JS
	StyleCSS       template.CSS
	AppJS          template.JS
}

var nonAlphanumericRegexp = regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)

// SanitizeTitle transforms a title string into a safe directory path component.
func SanitizeTitle(title string) string {
	cleaned := strings.TrimSpace(title)
	if cleaned == "" {
		return ""
	}
	cleaned = strings.ReplaceAll(cleaned, " ", "_")
	cleaned = nonAlphanumericRegexp.ReplaceAllString(cleaned, "")
	return strings.Trim(cleaned, "_-")
}

// Generate creates a versioned report folder, renders index.html, and updates the history catalog.
func Generate(comp differ.ComparisonReport, outputDir string) (string, error) {
	if comp.Timestamp.IsZero() {
		comp.Timestamp = time.Now()
	}

	timeStr := comp.Timestamp.Format("2006-01-02_15-04-05")
	sanitizedTitle := SanitizeTitle(comp.Title)

	var runDirName string
	if sanitizedTitle != "" {
		runDirName = fmt.Sprintf("run_%s_%s", timeStr, sanitizedTitle)
	} else {
		runDirName = fmt.Sprintf("run_%s", timeStr)
	}

	if comp.ID == "" {
		comp.ID = runDirName
	}
	if comp.Title == "" {
		comp.Title = runDirName
	}

	targetDir := filepath.Join(outputDir, runDirName)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("creating report directory %s: %w", targetDir, err)
	}

	reportJSONBytes, err := json.Marshal(comp)
	if err != nil {
		return "", fmt.Errorf("marshaling comparison report: %w", err)
	}

	styleContent, err := assetsFS.ReadFile("assets/style.css")
	if err != nil {
		return "", fmt.Errorf("reading style asset: %w", err)
	}

	appContent, err := assetsFS.ReadFile("assets/app.js")
	if err != nil {
		return "", fmt.Errorf("reading app asset: %w", err)
	}

	reportTmplContent, err := assetsFS.ReadFile("assets/report.html")
	if err != nil {
		return "", fmt.Errorf("reading report template: %w", err)
	}

	tmpl, err := template.New("report").Parse(string(reportTmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing report template: %w", err)
	}

	indexPath := filepath.Join(targetDir, "index.html")
	f, err := os.Create(indexPath)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", indexPath, err)
	}
	defer f.Close()

	tmplData := reportTemplateData{
		Title:          comp.Title,
		LeftPath:       comp.LeftPath,
		RightPath:      comp.RightPath,
		Timestamp:      comp.Timestamp.Format(time.RFC3339),
		TotalFiles:     comp.TotalFiles,
		AddedCount:     comp.AddedCount,
		DeletedCount:   comp.DeletedCount,
		ModifiedCount:  comp.ModifiedCount,
		IdenticalCount: comp.IdenticalCount,
		ReportJSON:     template.JS(reportJSONBytes),
		StyleCSS:       template.CSS(styleContent),
		AppJS:          template.JS(appContent),
	}

	if err := tmpl.Execute(f, tmplData); err != nil {
		return "", fmt.Errorf("executing report template: %w", err)
	}

	// Update Master Index Catalog
	meta := RunMetadata{
		ID:            comp.ID,
		DirectoryName: runDirName,
		Timestamp:     comp.Timestamp,
		FormattedDate: comp.Timestamp.Format("2006-01-02 15:04:05"),
		Title:         comp.Title,
		LeftPath:      comp.LeftPath,
		RightPath:      comp.RightPath,
		TotalFiles:    comp.TotalFiles,
		AddedCount:    comp.AddedCount,
		DeletedCount:  comp.DeletedCount,
		ModifiedCount: comp.ModifiedCount,
		RelativeURL:   filepath.ToSlash(filepath.Join(runDirName, "index.html")),
	}

	if err := UpdateHistoryCatalog(outputDir, meta); err != nil {
		return "", fmt.Errorf("updating history catalog: %w", err)
	}

	return indexPath, nil
}
