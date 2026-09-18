# kdiff

`kdiff` is a fast directory comparison and interactive HTML diff report CLI tool written in Go.

## Features

- **Fast Directory Scanning & Diffing:** Compares directory trees, detecting added, deleted, modified, identical, and binary files.
- **Self-Contained Interactive HTML Reports:** Generates standalone, responsive HTML diff reports with embedded side-by-side or inline views and statistics.
- **Run History & Catalog:** Automatically tracks previous comparison runs in a local history catalog (`~/.kdiff`).
- **Standard & Full Modes:** By default excludes noisy folders (`node_modules`, `vendor`, `.git`, build output), with `--full` / `-f` flag for exhaustive scans.
- **Multi-Platform Support:** Ready-to-use binaries for macOS (Apple Silicon & Intel), Linux (x86_64 & ARM64), and Windows.

---

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap ivanlunardi/tap
brew trust --formula ivanlunardi/tap/kdiff
brew install kdiff
```

### Go Install

```bash
go install github.com/ivanlunardi/kdiff@latest
```

### Pre-Built Binaries

Download pre-compiled binaries for macOS, Linux, and Windows from the [GitHub Releases](https://github.com/ivanlunardi/kdiff/releases) page.

---

## Usage

```bash
kdiff <command> [arguments]
```

### Available Commands

- `kdiff compare [flags] <left_dir> <right_dir>`: Compare two directories and generate an interactive HTML diff report.
  - `-f, --full`: Exhaustive scan including `node_modules`, `vendor`, build folders, etc.
  - `-o, --output <dir>`: Custom output directory for reports (default `~/.kdiff`).
  - `-t, --title <title>`: Custom title for this comparison run.
- `kdiff history [flags]`: List past comparison runs and catalog link.
  - `-o, --output <dir>`: Custom output directory (default `~/.kdiff`).
- `kdiff serve [flags]`: Start an HTTP server to view past comparison reports and catalog.
  - `-p, --port <port>`: Port to listen on (default `8080`).
  - `-H, --host <host>`: Host address to bind to (default `127.0.0.1`).
  - `-o, --output <dir>`: Custom output directory (default `~/.kdiff`).
- `kdiff clear [flags]`: Delete all past comparison reports and reset the catalog.
  - `-y, --yes`: Bypass interactive confirmation prompt.
  - `-o, --output <dir>`: Custom output directory (default `~/.kdiff`).
- `kdiff version`: Show `kdiff` version, commit hash, and build date.
- `kdiff help [command]`: Show help information for `kdiff` or a specific command.

### Examples

```bash
# Basic comparison
kdiff compare ./project-v1 ./project-v2

# Full comparison with custom title and output folder
kdiff compare --full -t "Release 2.0 vs 2.1" -o ./diff-reports ./dirA ./dirB

# View history
kdiff history

# Serve history and reports locally via HTTP
kdiff serve
kdiff serve -p 3000 -o ./diff-reports

# Check version
kdiff version
kdiff --version
```

---

## License

[MIT](LICENSE)
