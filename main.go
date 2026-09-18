package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ivanlunardi/kdiff/differ"
	"github.com/ivanlunardi/kdiff/report"
	"github.com/ivanlunardi/kdiff/scanner"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "dirty"
)

func getVersion() string {
	if version != "dev" && version != "" {
		return fmt.Sprintf("kdiff version %s (commit: %s, built at: %s)", version, commit, date)
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return fmt.Sprintf("kdiff version %s", info.Main.Version)
	}
	return fmt.Sprintf("kdiff version dev (commit: %s, built at: %s)", commit, date)
}

func main() {
	if err := Run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getDefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "./.kdiff"
	}
	return filepath.Join(home, ".kdiff")
}

func expandHomePath(path string) string {
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return home
		}
	}
	return path
}

func formatFileURL(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	slashPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return "file://" + slashPath
}

func printUsage(out io.Writer) {
	fmt.Fprintf(out, `kdiff - Directory comparison and versioned HTML diff report tool

Usage:
  kdiff <command> [arguments]

Available Commands:
  compare     Compare two directories and generate an HTML diff report
  history     List past comparison runs
  clear       Delete all past comparison reports and reset history
  version     Show kdiff version information
  help        Show help for kdiff or a specific command

Flags:
  -v, --version  Show version information
  -h, --help     Show help information

Use "kdiff help <command>" or "kdiff <command> --help" for more information about a command.
`)
}

// Run executes the kdiff CLI pipeline with given arguments and streams.
func Run(args []string, stdout, stderr io.Writer, stdin io.Reader) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		if len(cmdArgs) == 0 {
			printUsage(stdout)
			return nil
		}
		switch cmdArgs[0] {
		case "compare":
			return runCompare([]string{"--help"}, stdout, stderr)
		case "history":
			return runHistory([]string{"--help"}, stdout, stderr)
		case "clear":
			return runClear([]string{"--help"}, stdout, stderr, stdin)
		case "version":
			return runVersion([]string{"--help"}, stdout, stderr)
		default:
			fmt.Fprintf(stderr, "Unknown help command %q\n\n", cmdArgs[0])
			printUsage(stderr)
			return fmt.Errorf("unknown help command %q", cmdArgs[0])
		}
	case "compare":
		return runCompare(cmdArgs, stdout, stderr)
	case "history":
		return runHistory(cmdArgs, stdout, stderr)
	case "clear":
		return runClear(cmdArgs, stdout, stderr, stdin)
	case "version", "-v", "--version", "-V":
		return runVersion(cmdArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown command %q for \"kdiff\"\nRun 'kdiff --help' for usage.\n", cmd)
		return fmt.Errorf("unknown command %q for \"kdiff\"", cmd)
	}
}

func runVersion(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kdiff version", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kdiff version\n\n")
		fmt.Fprintf(stderr, "Show kdiff version, commit hash, and build date.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fmt.Fprintf(stderr, "  -h, --help   Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	fmt.Fprintln(stdout, getVersion())
	return nil
}

func runCompare(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kdiff compare", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var full bool
	fs.BoolVar(&full, "full", false, "Exhaustive scan: include node_modules, vendor, and build folders")
	fs.BoolVar(&full, "f", false, "Alias for --full")

	var outputDir string
	fs.StringVar(&outputDir, "output", "", "Output directory for reports (default \"~/.kdiff\")")
	fs.StringVar(&outputDir, "o", "", "Alias for --output")

	var title string
	fs.StringVar(&title, "title", "", "Custom title for this comparison run")
	fs.StringVar(&title, "t", "", "Alias for --title")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kdiff compare [flags] <left_dir> <right_dir>\n\n")
		fmt.Fprintf(stderr, "Compare two directories and generate an interactive versioned HTML diff report.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fmt.Fprintf(stderr, "  -f, --full            Exhaustive scan (do not exclude node_modules, vendor, .git, etc.)\n")
		fmt.Fprintf(stderr, "  -o, --output string   Output directory for reports (default \"~/.kdiff\")\n")
		fmt.Fprintf(stderr, "  -t, --title string    Custom title for this comparison run\n")
		fmt.Fprintf(stderr, "  -h, --help            Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	posArgs := fs.Args()
	if len(posArgs) < 2 {
		fs.Usage()
		return fmt.Errorf("both <left_dir> and <right_dir> arguments are required")
	}

	leftDir := posArgs[0]
	rightDir := posArgs[1]

	leftAbs, err := filepath.Abs(leftDir)
	if err != nil {
		return fmt.Errorf("resolving left path %s: %w", leftDir, err)
	}

	rightAbs, err := filepath.Abs(rightDir)
	if err != nil {
		return fmt.Errorf("resolving right path %s: %w", rightDir, err)
	}

	leftInfo, err := os.Stat(leftAbs)
	if err != nil {
		return fmt.Errorf("left directory does not exist or cannot be accessed: %s", leftDir)
	}
	if !leftInfo.IsDir() {
		return fmt.Errorf("left path is not a directory: %s", leftDir)
	}

	rightInfo, err := os.Stat(rightAbs)
	if err != nil {
		return fmt.Errorf("right directory does not exist or cannot be accessed: %s", rightDir)
	}
	if !rightInfo.IsDir() {
		return fmt.Errorf("right path is not a directory: %s", rightDir)
	}

	if title == "" {
		title = fmt.Sprintf("%s vs %s", filepath.Base(leftAbs), filepath.Base(rightAbs))
	}

	fmt.Fprintf(stdout, "Comparing:\n  Left:  %s\n  Right: %s\n", leftAbs, rightAbs)
	if full {
		fmt.Fprintf(stdout, "Mode: Full comparison (all folders included)\n")
	} else {
		fmt.Fprintf(stdout, "Mode: Standard comparison (excluding node_modules, vendor, build folders)\n")
	}

	// 1. Scan
	scannedItems, err := scanner.Scan(leftAbs, rightAbs, scanner.Options{Full: full})
	if err != nil {
		return fmt.Errorf("scanning directories: %w", err)
	}

	// 2. Differ
	fileDiffs := make([]differ.FileDiff, 0, len(scannedItems))
	var addedCount, deletedCount, modifiedCount, identicalCount int

	for _, item := range scannedItems {
		fd, err := differ.DiffFile(differ.ScannedInput{
			RelativePath: item.RelativePath,
			LeftPath:     item.LeftPath,
			RightPath:    item.RightPath,
			Status:       item.Status,
			IsBinary:     item.IsBinary,
			LeftSize:     item.LeftSize,
			RightSize:    item.RightSize,
		})
		if err != nil {
			return fmt.Errorf("diffing %s: %w", item.RelativePath, err)
		}

		switch item.Status {
		case differ.StatusAdded:
			addedCount++
		case differ.StatusDeleted:
			deletedCount++
		case differ.StatusModified, differ.StatusBinary:
			modifiedCount++
		case differ.StatusIdentical:
			identicalCount++
		}

		fileDiffs = append(fileDiffs, fd)
	}

	compReport := differ.ComparisonReport{
		Timestamp:      time.Now(),
		Title:          title,
		LeftPath:       leftAbs,
		RightPath:      rightAbs,
		TotalFiles:     len(fileDiffs),
		AddedCount:     addedCount,
		DeletedCount:   deletedCount,
		ModifiedCount:  modifiedCount,
		IdenticalCount: identicalCount,
		Files:          fileDiffs,
	}

	// 3. Generate Report
	if outputDir == "" {
		outputDir = getDefaultOutputDir()
	} else {
		outputDir = expandHomePath(outputDir)
	}

	outDirAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	reportFile, err := report.Generate(compReport, outDirAbs)
	if err != nil {
		return fmt.Errorf("generating report: %w", err)
	}

	catalogFile := filepath.Join(outDirAbs, "index.html")

	fmt.Fprintf(stdout, "\nComparison Summary:\n")
	fmt.Fprintf(stdout, "  Total Files: %d\n", len(fileDiffs))
	fmt.Fprintf(stdout, "  Modified:    %d\n", modifiedCount)
	fmt.Fprintf(stdout, "  Added:       %d\n", addedCount)
	fmt.Fprintf(stdout, "  Deleted:     %d\n", deletedCount)
	fmt.Fprintf(stdout, "  Identical:   %d\n", identicalCount)
	fmt.Fprintf(stdout, "\nReport generated:\n  %s\n", formatFileURL(reportFile))
	fmt.Fprintf(stdout, "Catalog updated:\n  %s\n", formatFileURL(catalogFile))

	return nil
}

func runHistory(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kdiff history", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var outputDir string
	fs.StringVar(&outputDir, "output", "", "Output directory for reports (default \"~/.kdiff\")")
	fs.StringVar(&outputDir, "o", "", "Alias for --output")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kdiff history [flags]\n\n")
		fmt.Fprintf(stderr, "List past comparison runs.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fmt.Fprintf(stderr, "  -o, --output string   Output directory for reports (default \"~/.kdiff\")\n")
		fmt.Fprintf(stderr, "  -h, --help            Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if outputDir == "" {
		outputDir = getDefaultOutputDir()
	} else {
		outputDir = expandHomePath(outputDir)
	}

	outDirAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	runs, err := report.LoadHistory(outDirAbs)
	if err != nil {
		return fmt.Errorf("loading history from %s: %w", outDirAbs, err)
	}

	if len(runs) == 0 {
		fmt.Fprintf(stdout, "No comparison history found in %s\n", outDirAbs)
		return nil
	}

	catalogFile := filepath.Join(outDirAbs, "index.html")
	fmt.Fprintf(stdout, "Comparison History (%d runs):\n", len(runs))
	fmt.Fprintf(stdout, "Catalog: %s\n\n", formatFileURL(catalogFile))

	for _, r := range runs {
		reportPath := filepath.Join(outDirAbs, r.DirectoryName, "index.html")
		fmt.Fprintf(stdout, "[%s] %s\n", r.FormattedDate, r.Title)
		fmt.Fprintf(stdout, "  Left:    %s\n", r.LeftPath)
		fmt.Fprintf(stdout, "  Right:   %s\n", r.RightPath)
		fmt.Fprintf(stdout, "  Changes: +%d added, ~%d modified, -%d deleted (%d total files)\n", r.AddedCount, r.ModifiedCount, r.DeletedCount, r.TotalFiles)
		fmt.Fprintf(stdout, "  Report:  %s\n\n", formatFileURL(reportPath))
	}

	return nil
}

func runClear(args []string, stdout, stderr io.Writer, stdin io.Reader) error {
	fs := flag.NewFlagSet("kdiff clear", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var outputDir string
	fs.StringVar(&outputDir, "output", "", "Output directory for reports (default \"~/.kdiff\")")
	fs.StringVar(&outputDir, "o", "", "Alias for --output")

	var yes bool
	fs.BoolVar(&yes, "yes", false, "Bypass interactive confirmation prompt")
	fs.BoolVar(&yes, "y", false, "Alias for --yes")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kdiff clear [flags]\n\n")
		fmt.Fprintf(stderr, "Delete all past comparison reports and reset the history catalog.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fmt.Fprintf(stderr, "  -o, --output string   Output directory for reports (default \"~/.kdiff\")\n")
		fmt.Fprintf(stderr, "  -y, --yes             Bypass interactive confirmation prompt\n")
		fmt.Fprintf(stderr, "  -h, --help            Show this help message\n")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if outputDir == "" {
		outputDir = getDefaultOutputDir()
	} else {
		outputDir = expandHomePath(outputDir)
	}

	outDirAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	if !yes {
		fmt.Fprintf(stdout, "Are you sure you want to delete all reports in %s? (y/N): ", outDirAbs)
		if stdin == nil {
			fmt.Fprintln(stdout, "Aborted.")
			return nil
		}
		scanner := bufio.NewScanner(stdin)
		var answer string
		if scanner.Scan() {
			answer = strings.TrimSpace(scanner.Text())
		}
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			fmt.Fprintln(stdout, "Aborted.")
			return nil
		}
	}

	cleanedCount, err := report.ClearHistory(outDirAbs)
	if err != nil {
		return fmt.Errorf("clearing history from %s: %w", outDirAbs, err)
	}

	fmt.Fprintf(stdout, "Successfully deleted %d comparison report(s) and reset history in %s\n", cleanedCount, outDirAbs)
	return nil
}
