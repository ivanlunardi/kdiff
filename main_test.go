package main_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		if !strings.Contains(outStr, "Available Commands:") || !strings.Contains(outStr, "compare") || !strings.Contains(outStr, "gitdiff") || !strings.Contains(outStr, "history") || !strings.Contains(outStr, "clear") || !strings.Contains(outStr, "version") {
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
		for _, sub := range []string{"compare", "gitdiff", "history", "serve", "clear", "version"} {
			var stdout, stderr bytes.Buffer
			err := main.Run([]string{"help", sub}, &stdout, &stderr, nil)
			if err != nil {
				t.Fatalf("unexpected error running kdiff help %s: %v", sub, err)
			}
			if !strings.Contains(stderr.String(), fmt.Sprintf("Usage: kdiff %s", sub)) {
				t.Errorf("expected %s help, got: %s", sub, stderr.String())
			}
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

	t.Run("Serve Subcommand - Help Flags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"serve", "--help"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff serve --help: %v", err)
		}
		if !strings.Contains(stderr.String(), "Usage: kdiff serve") {
			t.Errorf("expected serve help message in stderr, got: %s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "-p, --port") || !strings.Contains(stderr.String(), "-H, --host") {
			t.Errorf("expected flags in serve help message, got: %s", stderr.String())
		}
	})

	t.Run("Serve Subcommand - Serves Catalog and Reports", func(t *testing.T) {
		// Re-create a comparison report first
		var compareStdout, compareStderr bytes.Buffer
		err := main.Run([]string{"compare", "-o", reportsDir, "-t", "Serve Test Run", leftDir, rightDir}, &compareStdout, &compareStderr, nil)
		if err != nil {
			t.Fatalf("failed to create report: %v", err)
		}

		freePort := getFreePort(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var stdout, stderr bytes.Buffer
		errCh := make(chan error, 1)

		go func() {
			errCh <- main.RunContext(ctx, []string{"serve", "-p", fmt.Sprintf("%d", freePort), "-H", "127.0.0.1", "-o", reportsDir}, &stdout, &stderr, nil)
		}()

		baseURL := fmt.Sprintf("http://127.0.0.1:%d", freePort)
		client := &http.Client{Timeout: 1 * time.Second}

		// Wait for server to start
		var resp *http.Response
		var lastErr error
		for range 50 {
			time.Sleep(20 * time.Millisecond)
			resp, lastErr = client.Get(baseURL + "/")
			if lastErr == nil && resp.StatusCode == http.StatusOK {
				break
			}
		}

		if lastErr != nil || resp == nil || resp.StatusCode != http.StatusOK {
			cancel()
			<-errCh
			t.Fatalf("server failed to respond on %s: %v", baseURL, lastErr)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read index response: %v", err)
		}
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "Reports Catalog") || !strings.Contains(bodyStr, "Serve Test Run") {
			t.Errorf("expected catalog content with 'Reports Catalog' and 'Serve Test Run', got: %s", bodyStr)
		}

		// Verify history.json is served
		histResp, err := client.Get(baseURL + "/history.json")
		if err != nil {
			t.Fatalf("failed to get history.json: %v", err)
		}
		defer histResp.Body.Close()
		if histResp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for history.json, got: %d", histResp.StatusCode)
		}

		// Cancel context and verify graceful shutdown
		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("unexpected server error on shutdown: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for server to shut down")
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Starting kdiff report server...") {
			t.Errorf("expected starting message in stdout, got: %s", outStr)
		}
		if !strings.Contains(outStr, fmt.Sprintf("Available at: %s", baseURL)) {
			t.Errorf("expected available URL in stdout, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Server stopped.") {
			t.Errorf("expected 'Server stopped.' in stdout, got: %s", outStr)
		}
	})

	t.Run("Serve Subcommand - Empty reports directory creates catalog and serves", func(t *testing.T) {
		emptyServeDir := filepath.Join(tempDir, "fresh_empty_serve_dir")
		freePort := getFreePort(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var stdout, stderr bytes.Buffer
		errCh := make(chan error, 1)

		go func() {
			errCh <- main.RunContext(ctx, []string{"serve", "--port", fmt.Sprintf("%d", freePort), "--output", emptyServeDir}, &stdout, &stderr, nil)
		}()

		baseURL := fmt.Sprintf("http://127.0.0.1:%d", freePort)
		client := &http.Client{Timeout: 1 * time.Second}

		var resp *http.Response
		var lastErr error
		for range 50 {
			time.Sleep(20 * time.Millisecond)
			resp, lastErr = client.Get(baseURL + "/")
			if lastErr == nil && resp.StatusCode == http.StatusOK {
				break
			}
		}

		if lastErr != nil || resp == nil || resp.StatusCode != http.StatusOK {
			cancel()
			<-errCh
			t.Fatalf("server failed to respond on empty dir %s: %v", baseURL, lastErr)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}
		if !strings.Contains(string(body), "Reports Catalog") || !strings.Contains(string(body), "No comparison runs found.") {
			t.Errorf("expected empty catalog content, got: %s", string(body))
		}

		cancel()
		<-errCh
	})

	t.Run("Serve Subcommand - Invalid flag returns error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"serve", "--invalid-flag"}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error for invalid flag, got nil")
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

	t.Run("GitDiff Subcommand - Help and Flags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff", "--help"}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running kdiff gitdiff --help: %v", err)
		}
		if !strings.Contains(stderr.String(), "Usage: kdiff gitdiff") {
			t.Errorf("expected gitdiff help message in stderr, got: %s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "--hash") || !strings.Contains(stderr.String(), "--tag") {
			t.Errorf("expected --hash and --tag flags in help, got: %s", stderr.String())
		}
	})

	t.Run("GitDiff Subcommand - Git not installed returns error", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir()) // empty directory so git is not found
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff"}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error when git is not in PATH, got nil")
		}
		if !strings.Contains(err.Error(), "git is not installed or not in PATH") {
			t.Errorf("expected error about git not installed, got: %v", err)
		}
	})

	t.Run("GitDiff Subcommand - Non-git folder returns error", func(t *testing.T) {
		nonGitDir := t.TempDir()
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff", nonGitDir}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error when running gitdiff on non-git directory, got nil")
		}
		if !strings.Contains(err.Error(), "not inside a git repository") {
			t.Errorf("expected error mentioning not inside a git repository, got: %v", err)
		}
	})

	t.Run("GitDiff Subcommand - Both --hash and --tag specified returns error", func(t *testing.T) {
		gitDir := t.TempDir()
		initTestGitRepo(t, gitDir)

		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff", "--hash", "abc", "--tag", "v1.0.0", gitDir}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error specifying both --hash and --tag, got nil")
		}
		if !strings.Contains(err.Error(), "cannot specify both --hash and --tag") {
			t.Errorf("expected error about both flags, got: %v", err)
		}
	})

	t.Run("GitDiff Subcommand - Unborn HEAD in empty repo returns error", func(t *testing.T) {
		emptyGitDir := t.TempDir()
		runGitCmd(t, emptyGitDir, "init")

		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff", emptyGitDir}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error on empty git repo with unborn HEAD, got nil")
		}
		if !strings.Contains(err.Error(), "no commits") && !strings.Contains(err.Error(), "HEAD is unborn") {
			t.Errorf("expected error about no commits / unborn HEAD, got: %v", err)
		}
	})

	t.Run("GitDiff Subcommand - Non-existent tag or hash returns error", func(t *testing.T) {
		gitDir := t.TempDir()
		initTestGitRepo(t, gitDir)

		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff", "--tag", "non-existent-tag", gitDir}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error for non-existent tag, got nil")
		}
		if !strings.Contains(err.Error(), "tag not found") {
			t.Errorf("expected 'tag not found' error, got: %v", err)
		}

		stderr.Reset()
		stdout.Reset()
		err = main.Run([]string{"gitdiff", "--hash", "deadbeef00", gitDir}, &stdout, &stderr, nil)
		if err == nil {
			t.Fatal("expected error for non-existent hash, got nil")
		}
		if !strings.Contains(err.Error(), "commit hash not found") {
			t.Errorf("expected 'commit hash not found' error, got: %v", err)
		}
	})

	t.Run("GitDiff Subcommand - HEAD default, Tag, Hash, and Subdirectory comparisons", func(t *testing.T) {
		gitRepo := t.TempDir()
		firstHash, secondHash := initTestGitRepo(t, gitRepo)

		// Create a subdirectory inside the repo with some files
		subDir := filepath.Join(gitRepo, "pkg", "sub")
		mustMkdir(t, subDir)
		mustWriteFile(t, filepath.Join(subDir, "code.go"), "package sub\n\nfunc V1() {}\n")
		runGitCmd(t, gitRepo, "add", ".")
		runGitCmd(t, gitRepo, "commit", "-m", "add pkg/sub")
		thirdHash := strings.TrimSpace(runGitCmd(t, gitRepo, "rev-parse", "HEAD"))
		runGitCmd(t, gitRepo, "tag", "v3.0.0")

		// Make uncommitted working tree modifications:
		// 1. Modify file1.txt
		mustWriteFile(t, filepath.Join(gitRepo, "file1.txt"), "hello v3 working tree\n")
		// 2. Add untracked file4.txt
		mustWriteFile(t, filepath.Join(gitRepo, "file4.txt"), "brand new untracked\n")
		// 3. Delete file3.txt
		_ = os.Remove(filepath.Join(gitRepo, "file3.txt"))
		// 4. Modify sub/code.go
		mustWriteFile(t, filepath.Join(subDir, "code.go"), "package sub\n\nfunc V2() {}\n")

		gitReportsDir := filepath.Join(tempDir, "git_reports")

		// Test 1: Compare against HEAD (default)
		var stdout, stderr bytes.Buffer
		err := main.Run([]string{"gitdiff", "-o", gitReportsDir, gitRepo}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running gitdiff against HEAD: %v, stderr: %s", err, stderr.String())
		}

		outStr := stdout.String()
		if !strings.Contains(outStr, "Comparing:") || !strings.Contains(outStr, "git:HEAD") {
			t.Errorf("expected output to mention git:HEAD, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Comparison Summary:") {
			t.Errorf("expected summary in output, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Modified:    2") { // file1.txt, pkg/sub/code.go
			t.Errorf("expected 2 modified files against HEAD, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Added:       1") { // file4.txt
			t.Errorf("expected 1 added file against HEAD, got: %s", outStr)
		}
		if !strings.Contains(outStr, "Deleted:     1") { // file3.txt
			t.Errorf("expected 1 deleted file against HEAD, got: %s", outStr)
		}

		// Test 2: Compare against tag v1.0.0
		stdout.Reset()
		stderr.Reset()
		err = main.Run([]string{"gitdiff", "--tag", "v1.0.0", "-o", gitReportsDir, "-t", "Diff against v1", gitRepo}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running gitdiff against tag: %v, stderr: %s", err, stderr.String())
		}
		outStr = stdout.String()
		if !strings.Contains(outStr, "git:tag v1.0.0") {
			t.Errorf("expected output to mention git:tag v1.0.0, got: %s", outStr)
		}

		// Test 3: Compare against commit hash (second commit)
		stdout.Reset()
		stderr.Reset()
		err = main.Run([]string{"gitdiff", "--hash", secondHash, "-o", gitReportsDir, gitRepo}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running gitdiff against hash %s: %v, stderr: %s", secondHash, err, stderr.String())
		}
		outStr = stdout.String()
		if !strings.Contains(outStr, "git:commit") {
			t.Errorf("expected output to mention git:commit, got: %s", outStr)
		}

		// Test 4: Run gitdiff on subdirectory
		stdout.Reset()
		stderr.Reset()
		err = main.Run([]string{"gitdiff", "-o", gitReportsDir, subDir}, &stdout, &stderr, nil)
		if err != nil {
			t.Fatalf("unexpected error running gitdiff on subdir: %v, stderr: %s", err, stderr.String())
		}
		outStr = stdout.String()
		if !strings.Contains(outStr, "Total Files: 1") || !strings.Contains(outStr, "Modified:    1") {
			t.Errorf("expected 1 total file modified in subdir diff, got: %s", outStr)
		}

		_ = firstHash
		_ = thirdHash
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

func initTestGitRepo(t *testing.T, dir string) (firstCommitHash, secondCommitHash string) {
	t.Helper()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.name", "Kdiff Tester")
	runGitCmd(t, dir, "config", "user.email", "tester@kdiff.test")
	runGitCmd(t, dir, "config", "commit.gpgsign", "false")

	// Commit 1
	mustWriteFile(t, filepath.Join(dir, "file1.txt"), "hello v1\n")
	mustWriteFile(t, filepath.Join(dir, "file2.txt"), "to be deleted\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "initial commit")
	firstCommitHash = strings.TrimSpace(runGitCmd(t, dir, "rev-parse", "HEAD"))
	runGitCmd(t, dir, "tag", "v1.0.0")

	// Commit 2
	mustWriteFile(t, filepath.Join(dir, "file1.txt"), "hello v2\n")
	runGitCmd(t, dir, "rm", "file2.txt")
	mustWriteFile(t, filepath.Join(dir, "file3.txt"), "added in commit 2\n")
	runGitCmd(t, dir, "add", ".")
	runGitCmd(t, dir, "commit", "-m", "second commit")
	secondCommitHash = strings.TrimSpace(runGitCmd(t, dir, "rev-parse", "HEAD"))
	runGitCmd(t, dir, "tag", "v2.0.0")

	return firstCommitHash, secondCommitHash
}

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git command %v in %s failed: %v, output: %s", args, dir, err, string(out))
	}
	return string(out)
}

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
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
