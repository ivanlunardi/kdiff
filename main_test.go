package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	main "github.com/ivanlunardi/kdiff"
)

func TestCLIIntegration(t *testing.T) {
	tempDir := t.TempDir()
	leftDir := filepath.Join(tempDir, "left")
	rightDir := filepath.Join(tempDir, "right")
	reportsDir := filepath.Join(tempDir, "custom_reports")

	// Setup directories
	mustMkdir(t, filepath.Join(leftDir, "src"))
	mustMkdir(t, filepath.Join(rightDir, "src"))
	mustMkdir(t, filepath.Join(leftDir, "node_modules"))
	mustMkdir(t, filepath.Join(rightDir, "node_modules"))

	mustWriteFile(t, filepath.Join(leftDir, "src", "app.js"), "const a = 1;\nconsole.log(a);\n")
	mustWriteFile(t, filepath.Join(rightDir, "src", "app.js"), "const a = 2;\nconsole.log(a);\nconst b = 3;\n")

	mustWriteFile(t, filepath.Join(leftDir, "src", "old.txt"), "delete me\n")
	mustWriteFile(t, filepath.Join(rightDir, "src", "new.txt"), "add me\n")

	mustWriteFile(t, filepath.Join(leftDir, "node_modules", "dep.js"), "dep v1")
	mustWriteFile(t, filepath.Join(rightDir, "node_modules", "dep.js"), "dep v2")

	t.Run("Zero Arguments Displays General Usage", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff with zero args: %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Available Commands:") || !strings.Contains(outStr, "compare") || !strings.Contains(outStr, "history") || !strings.Contains(outStr, "clear") || !strings.Contains(outStr, "version") {
			t.Errorf("expected general usage with commands including version, got: %s", outStr)
		}
		if !strings.Contains(outStr, "-v, --version") {
			t.Errorf("expected -v, --version in general flags, got: %s", outStr)
		}
	})

	t.Run("Help Flag Displays General Usage", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"--help"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff --help: %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Available Commands:") {
			t.Errorf("expected general usage in stdout, got: %s", outStr)
		}
	})

	t.Run("Help Command with Subcommands", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"help", "compare"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff help compare: %v", err)
		}
		if !strings.Contains(stderr.String(), "Usage: kdiff compare") {
			t.Errorf("expected compare help, got: %s", stderr.String())
		}
	})

	t.Run("Compare Subcommand - Standard Run with Output and Title Flags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		args := []string{
			"compare",
			"-o", reportsDir,
			"-t", "Integration Test Run",
			leftDir, rightDir,
		}

		err := main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected CLI error: %v, stderr: %s", err, stderr.String())
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Comparison Summary:") {
			t.Errorf("expected summary in stdout, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Total Files: 3") { // excluding node_modules/dep.js
			t.Errorf("expected 3 files compared in standard mode, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Report generated:\n  file://") {
			t.Errorf("expected 'Report generated:\\n  file://' in output, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Catalog updated:\n  file://") {
			t.Errorf("expected 'Catalog updated:\\n  file://' in output, got: %s", outStr)
		}

		// Verify catalog created
		catalogPath := filepath.Join(reportsDir, "index.html")
		if _, err := os.Stat(catalogPath); err != nil {
			t.Fatalf("catalog index.html not found: %v", err)
		}
	})

	t.Run("History Subcommand Displays Runs When Present", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		args := []string{"history", "-o", reportsDir}

		err := main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff history: %v, stderr: %s", err, stderr.String())
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Comparison History") {
			t.Errorf("expected 'Comparison History' header, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Integration Test Run") {
			t.Errorf("expected 'Integration Test Run' in history output, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Report:  file://") {
			t.Errorf("expected 'Report:  file://' link in history output, got: %s", outStr)
		}
	})

	t.Run("History Subcommand Empty History", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty_reports")
		var stdout, stderr bytes.Buffer
		args := []string{"history", "-o", emptyDir}

		err := main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff history on empty dir: %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "No comparison history found") {
			t.Errorf("expected 'No comparison history found', got: %s", outStr)
		}
	})

	t.Run("Compare Subcommand - Default Output Directory to ~/.kdiff", func(t *testing.T) {
		mockHome := filepath.Join(tempDir, "mock_user_home")
		mustMkdir(t, mockHome)
		t.Setenv("HOME", mockHome)
		t.Setenv("USERPROFILE", mockHome)

		var stdout, stderr bytes.Buffer
		args := []string{"compare", leftDir, rightDir}

		err := main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected CLI error without output flag: %v, stderr: %s", err, stderr.String())
		}

		expectedKdiffDir := filepath.Join(mockHome, ".kdiff")
		catalogPath := filepath.Join(expectedKdiffDir, "index.html")
		if _, err := os.Stat(catalogPath); err != nil {
			t.Fatalf("expected catalog in ~/.kdiff (%s), but not found: %v", catalogPath, err)
		}
	})

	t.Run("Compare Subcommand - Tilde expansion in -o flag", func(t *testing.T) {
		mockHome := filepath.Join(tempDir, "mock_user_home_tilde")
		mustMkdir(t, mockHome)
		t.Setenv("HOME", mockHome)
		t.Setenv("USERPROFILE", mockHome)

		var stdout, stderr bytes.Buffer
		args := []string{
			"compare",
			"-o", "~/custom_reports_tilde",
			leftDir, rightDir,
		}

		err := main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected CLI error with tilde path: %v, stderr: %s", err, stderr.String())
		}

		expectedDir := filepath.Join(mockHome, "custom_reports_tilde")
		catalogPath := filepath.Join(expectedDir, "index.html")
		if _, err := os.Stat(catalogPath); err != nil {
			t.Fatalf("expected catalog in %s, but not found: %v", catalogPath, err)
		}
	})

	t.Run("Compare Subcommand - Full Run includes node_modules", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		args := []string{
			"compare",
			"--full",
			"-o", reportsDir,
			"-t", "Full Scan Run",
			leftDir, rightDir,
		}

		err := main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected CLI error with --full: %v, stderr: %s", err, stderr.String())
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Total Files: 4") { // including node_modules/dep.js
			t.Errorf("expected 4 files compared in full mode, got: %s", outStr)
		}
	})

	t.Run("Compare Subcommand - Missing arguments error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		args := []string{"compare", leftDir}

		err := main.Run(args, &stdout, &stderr, nil)
		if err == nil {
			t.Fatalf("expected error for missing arguments, got nil")
		}
	})

	t.Run("Clear Subcommand - Interactive Abort on 'n'", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		stdin := strings.NewReader("n\n")
		args := []string{"clear", "-o", reportsDir}

		err := main.Run(args, &stdout, &stderr, stdin)
		if err != nil {
			t.Fatalf("unexpected error during clear abort: %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Are you sure you want to delete") {
			t.Errorf("expected confirmation prompt, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Aborted.") {
			t.Errorf("expected 'Aborted.', got: %s", outStr)
		}

		// Verify files are not deleted
		catalogPath := filepath.Join(reportsDir, "index.html")
		if _, err := os.Stat(catalogPath); err != nil {
			t.Errorf("expected catalog index.html to remain after abort")
		}
	})

	t.Run("Clear Subcommand - Interactive Confirmation 'yes'", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		stdin := strings.NewReader("yes\n")
		args := []string{"clear", "-o", reportsDir}

		err := main.Run(args, &stdout, &stderr, stdin)
		if err != nil {
			t.Fatalf("unexpected error during clear confirm: %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Successfully deleted") {
			t.Errorf("expected success feedback, got: %s", outStr)
		}

		// Verify catalog is deleted
		catalogPath := filepath.Join(reportsDir, "index.html")
		if _, err := os.Stat(catalogPath); !os.IsNotExist(err) {
			t.Errorf("expected catalog index.html to be deleted")
		}
	})

	t.Run("Clear Subcommand - Interactive Confirmation 'y'", func(t *testing.T) {
		// Re-create a comparison report first
		var compareStdout, compareStderr bytes.Buffer
		err := main.Run([]string{"compare", "-o", reportsDir, leftDir, rightDir}, &compareStdout, &compareStderr, nil)
		if err != nil {
			t.Fatalf("failed to create report: %v", err)
		}

		var stdout, stderr bytes.Buffer
		stdin := strings.NewReader("y\n")
		args := []string{"clear", "-o", reportsDir}

		err = main.Run(args, &stdout, &stderr, stdin)
		if err != nil {
			t.Fatalf("unexpected error during clear confirm 'y': %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Successfully deleted") {
			t.Errorf("expected success feedback, got: %s", outStr)
		}
	})

	t.Run("Clear Subcommand - Interactive Abort on unrecognized input", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		stdin := strings.NewReader("maybe\n")
		args := []string{"clear", "-o", reportsDir}

		err := main.Run(args, &stdout, &stderr, stdin)
		if err != nil {
			t.Fatalf("unexpected error during clear abort: %v", err)
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Aborted.") {
			t.Errorf("expected 'Aborted.', got: %s", outStr)
		}
	})

	t.Run("Clear Subcommand - Flag --yes bypasses prompt", func(t *testing.T) {
		// Re-create a comparison report first
		var compareStdout, compareStderr bytes.Buffer
		err := main.Run([]string{"compare", "-o", reportsDir, leftDir, rightDir}, &compareStdout, &compareStderr, nil)
		if err != nil {
			t.Fatalf("failed to create report: %v", err)
		}

		var stdout, stderr bytes.Buffer
		args := []string{"clear", "-o", reportsDir, "--yes"}

		err = main.Run(args, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error during clear with --yes: %v", err)
		}

		outStr := stdout.String()
		if strings.Contains(outStr, "Are you sure") {
			t.Errorf("did not expect prompt with --yes flag, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Successfully deleted") {
			t.Errorf("expected success message, got: %s", outStr)
		}
	})

	t.Run("Version Command Displays Version", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"version"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff version: %v", err)
		}

		outStr := strings.TrimSpace(stdout.String())
		if !strings.HasPrefix(outStr, "kdiff version") {
			t.Errorf("expected version output to start with 'kdiff version', got: %s", outStr)
		}
	})

	t.Run("Version Flags Display Version", func(t *testing.T) {
		for _, flag := range []string{"-v", "--version", "-V"} {
			var stdout, stderr bytes.Buffer
			err := main.Run([]string{flag}, &stdout, &stderr, nil)
			if err != nil {
				t.Fatalf("unexpected error running kdiff %s: %v", flag, err)
			}

			outStr := strings.TrimSpace(stdout.String())
			if !strings.HasPrefix(outStr, "kdiff version") {
				t.Errorf("expected version output for flag %s, got: %s", flag, outStr)
			}
		}
	})

	t.Run("Version Help Command and Flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"help", "version"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff help version: %v", err)
		}
		if !strings.Contains(stderr.String(), "Usage: kdiff version") {
			t.Errorf("expected version help message in stderr, got: %s", stderr.String())
		}

		stderr.Reset()
		stdout.Reset()
		err = main.Run([]string{"version", "--help"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff version --help: %v", err)
		}
		if !strings.Contains(stderr.String(), "Usage: kdiff version") {
			t.Errorf("expected version help message in stderr, got: %s", stderr.String())
		}
	})

	t.Run("Unknown Subcommand Returns Error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		args := []string{"unknown-command"}

		err := main.Run(args, &stdout, &stderr, nil)
		if err == nil {
			t.Fatalf("expected error for unknown subcommand, got nil")
		}
		if !strings.Contains(stderr.String(), "Unknown command") {
			t.Errorf("expected error message in stderr, got: %s", stderr.String())
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

func TestVersionLdflags(t *testing.T) {
	tempBin := filepath.Join(t.TempDir(), "kdiff_test_bin")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version=1.2.3 -X main.commit=abcdef1 -X main.date=2026-09-18T12:00:00Z -X main.builtBy=test", "-o", tempBin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build test binary with ldflags: %v, output: %s", err, string(out))
	}

	for _, flag := range []string{"version", "-v", "--version", "-V"} {
		execCmd := exec.Command(tempBin, flag)
		binOut, err := execCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to run test binary %s: %v, output: %s", flag, err, string(binOut))
		}

		expected := "kdiff version 1.2.3 (commit: abcdef1, built at: 2026-09-18T12:00:00Z)"
		if strings.TrimSpace(string(binOut)) != expected {
			t.Errorf("for flag %s expected version output %q, got %q", flag, expected, strings.TrimSpace(string(binOut)))
		}
	}
}
