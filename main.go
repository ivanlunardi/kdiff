package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
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
  gitdiff     Compare directory with a git reference (HEAD, commit, tag)
  history     List past comparison runs
  serve       Start an HTTP server to view comparison reports
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return RunContext(ctx, args, stdout, stderr, stdin)
}

// RunContext executes the kdiff CLI pipeline with context, arguments, and streams.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer, stdin io.Reader) error {
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
		case "gitdiff":
			return runGitDiff(ctx, []string{"--help"}, stdout, stderr)
		case "history":
			return runHistory([]string{"--help"}, stdout, stderr)
		case "serve":
			return runServe(ctx, []string{"--help"}, stdout, stderr)
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
	case "gitdiff":
		return runGitDiff(ctx, cmdArgs, stdout, stderr)
	case "history":
		return runHistory(cmdArgs, stdout, stderr)
	case "serve":
		return runServe(ctx, cmdArgs, stdout, stderr)
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

	return executeComparison(leftAbs, rightAbs, leftAbs, rightAbs, title, outputDir, full, stdout)
}

func runGitDiff(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kdiff gitdiff", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var hash string
	fs.StringVar(&hash, "hash", "", "Git commit hash to compare against (defaults to HEAD)")

	var tag string
	fs.StringVar(&tag, "tag", "", "Git tag to compare against (defaults to HEAD)")

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
		fmt.Fprintf(stderr, "Usage: kdiff gitdiff [flags] [dir]\n\n")
		fmt.Fprintf(stderr, "Compare current directory or repository with a git reference (HEAD, commit hash, or tag) and generate an HTML diff report.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fmt.Fprintf(stderr, "      --hash string     Git commit hash to compare against\n")
		fmt.Fprintf(stderr, "      --tag string      Git tag to compare against\n")
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

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not in PATH: %w", err)
	}

	posArgs := fs.Args()
	targetDir := "."
	if len(posArgs) > 0 {
		targetDir = posArgs[0]
	}
	if len(posArgs) > 1 {
		fs.Usage()
		return fmt.Errorf("too many arguments provided for gitdiff")
	}

	if hash != "" && tag != "" {
		return fmt.Errorf("cannot specify both --hash and --tag")
	}

	targetAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving target directory %s: %w", targetDir, err)
	}

	info, err := os.Stat(targetAbs)
	if err != nil {
		return fmt.Errorf("directory does not exist or cannot be accessed: %s", targetDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", targetDir)
	}

	checkRepoCmd := exec.CommandContext(ctx, "git", "-C", targetAbs, "rev-parse", "--is-inside-work-tree")
	out, err := checkRepoCmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("%s is not inside a git repository", targetAbs)
	}

	repoRootCmd := exec.CommandContext(ctx, "git", "-C", targetAbs, "rev-parse", "--show-toplevel")
	rootOut, err := repoRootCmd.Output()
	if err != nil {
		return fmt.Errorf("getting git repository root: %w", err)
	}
	repoRoot := strings.TrimSpace(string(rootOut))

	prefixCmd := exec.CommandContext(ctx, "git", "-C", targetAbs, "rev-parse", "--show-prefix")
	prefixOut, err := prefixCmd.Output()
	if err != nil {
		return fmt.Errorf("getting git prefix: %w", err)
	}
	prefix := strings.TrimSpace(string(prefixOut))

	var gitRef string
	var refType string
	if hash != "" {
		gitRef = hash
		refType = "commit"
	} else if tag != "" {
		gitRef = tag
		refType = "tag"
	} else {
		gitRef = "HEAD"
		refType = "head"
	}

	// Verify git ref exists
	verifyCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", gitRef+"^{commit}")
	if _, err := verifyCmd.CombinedOutput(); err != nil {
		vCmd2 := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", gitRef)
		if _, err2 := vCmd2.CombinedOutput(); err2 != nil {
			switch refType {
			case "commit":
				return fmt.Errorf("commit hash not found: %s", hash)
			case "tag":
				return fmt.Errorf("tag not found: %s", tag)
			default:
				return fmt.Errorf("git repository has no commits (HEAD is unborn)")
			}
		}
	}

	// Get short commit SHA
	shortShaCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--short", gitRef)
	var shortSha string
	if shaOut, err := shortShaCmd.Output(); err == nil {
		shortSha = strings.TrimSpace(string(shaOut))
	} else {
		shortSha = gitRef
	}

	// Export git revision to temporary directory
	tempDir, err := os.MkdirTemp("", "kdiff-git-*")
	if err != nil {
		return fmt.Errorf("creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := extractGitArchive(ctx, repoRoot, gitRef, tempDir); err != nil {
		return fmt.Errorf("extracting git archive for %s: %w", gitRef, err)
	}

	leftPath := tempDir
	if prefix != "" {
		leftPath = filepath.Join(tempDir, filepath.FromSlash(prefix))
		if err := os.MkdirAll(leftPath, 0755); err != nil {
			return fmt.Errorf("preparing comparison directory: %w", err)
		}
	}

	baseName := filepath.Base(targetAbs)
	if title == "" {
		switch refType {
		case "commit":
			title = fmt.Sprintf("Commit %s vs %s", shortSha, baseName)
		case "tag":
			title = fmt.Sprintf("Tag %s vs %s", tag, baseName)
		default:
			title = fmt.Sprintf("HEAD (%s) vs %s", shortSha, baseName)
		}
	}

	var leftLabel string
	switch refType {
	case "commit":
		leftLabel = fmt.Sprintf("git:commit %s", shortSha)
	case "tag":
		leftLabel = fmt.Sprintf("git:tag %s (%s)", tag, shortSha)
	default:
		leftLabel = fmt.Sprintf("git:HEAD (%s)", shortSha)
	}

	return executeComparison(leftPath, targetAbs, leftLabel, targetAbs, title, outputDir, full, stdout)
}

func extractGitArchive(ctx context.Context, repoRoot, gitRef, destDir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "archive", "--format=tar", gitRef)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting git archive: %w", err)
	}

	tarReader := tar.NewReader(stdout)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = cmd.Wait()
			return fmt.Errorf("reading tar stream: %w", err)
		}

		cleanPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			continue
		}

		targetPath := filepath.Join(destDir, cleanPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				_ = cmd.Wait()
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				_ = cmd.Wait()
				return err
			}
			mode := header.FileInfo().Mode()
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
			if err != nil {
				_ = cmd.Wait()
				return err
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				_ = cmd.Wait()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				_ = cmd.Wait()
				return err
			}
			_ = os.Remove(targetPath)
			_ = os.Symlink(header.Linkname, targetPath)
		}
	}

	if err := cmd.Wait(); err != nil {
		errOutput := strings.TrimSpace(stderrBuf.String())
		if errOutput != "" {
			return fmt.Errorf("git archive failed: %s", errOutput)
		}
		return fmt.Errorf("git archive failed: %w", err)
	}
	return nil
}

func executeComparison(leftScanDir, rightScanDir, leftDisplay, rightDisplay, title, outputDir string, full bool, stdout io.Writer) error {
	fmt.Fprintf(stdout, "Comparing:\n  Left:  %s\n  Right: %s\n", leftDisplay, rightDisplay)
	if full {
		fmt.Fprintf(stdout, "Mode: Full comparison (all folders included)\n")
	} else {
		fmt.Fprintf(stdout, "Mode: Standard comparison (excluding node_modules, vendor, build folders)\n")
	}

	// 1. Scan
	scannedItems, err := scanner.Scan(leftScanDir, rightScanDir, scanner.Options{Full: full})
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
		LeftPath:       leftDisplay,
		RightPath:      rightDisplay,
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

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kdiff serve", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var port int
	fs.IntVar(&port, "port", 8080, "Port to listen on (default 8080)")
	fs.IntVar(&port, "p", 8080, "Alias for --port")

	var host string
	fs.StringVar(&host, "host", "127.0.0.1", "Host address to bind to (default \"127.0.0.1\")")
	fs.StringVar(&host, "H", "127.0.0.1", "Alias for --host")

	var outputDir string
	fs.StringVar(&outputDir, "output", "", "Output directory for reports (default \"~/.kdiff\")")
	fs.StringVar(&outputDir, "o", "", "Alias for --output")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kdiff serve [flags]\n\n")
		fmt.Fprintf(stderr, "Start an HTTP server to view past comparison reports and catalog.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fmt.Fprintf(stderr, "  -p, --port int        Port to listen on (default 8080)\n")
		fmt.Fprintf(stderr, "  -H, --host string     Host address to bind to (default \"127.0.0.1\")\n")
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

	if err := os.MkdirAll(outDirAbs, 0755); err != nil {
		return fmt.Errorf("creating reports directory %s: %w", outDirAbs, err)
	}

	if err := report.EnsureCatalog(outDirAbs); err != nil {
		return fmt.Errorf("initializing catalog in %s: %w", outDirAbs, err)
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("starting listener on %s: %w", addr, err)
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(outDirAbs)))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	actualPort := port
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		actualPort = tcpAddr.Port
	}

	serverURL := fmt.Sprintf("http://%s:%d", host, actualPort)
	if host == "0.0.0.0" || host == "" {
		serverURL = fmt.Sprintf("http://localhost:%d", actualPort)
	}

	fmt.Fprintf(stdout, "Starting kdiff report server...\n")
	fmt.Fprintf(stdout, "Serving reports from: %s\n", outDirAbs)
	fmt.Fprintf(stdout, "Available at: %s\n", serverURL)
	fmt.Fprintf(stdout, "Press Ctrl+C to stop.\n")

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		fmt.Fprintln(stdout, "\nShutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		fmt.Fprintln(stdout, "Server stopped.")
		return nil
	}
}
