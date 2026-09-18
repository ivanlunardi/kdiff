#!/usr/bin/env bash

set -euo pipefail

# Print usage / help
show_help() {
    cat <<EOF
Usage: $(basename "$0") [VERSION] [OPTIONS]

Automates the release process for kdiff:
  1. Validates or computes the next release version (semver).
  2. If VERSION is omitted, increments the patch version of the latest tag (e.g., v1.0.1 -> v1.0.2).
  3. If VERSION is specified, ensures it is strictly greater than the latest tag.
  4. Commits any pending changes (if present) and pushes the current branch to origin.
  5. Creates an annotated Git tag and pushes it to origin to trigger the release workflow.

Arguments:
  VERSION         Optional target version (e.g., 1.0.2 or v1.0.2).

Options:
  -d, --dry-run   Simulate the release process without making commits, tags, or pushes.
  -h, --help      Display this help message.

Examples:
  $(basename "$0")               # Auto-increment patch (e.g. v1.0.1 -> v1.0.2)
  $(basename "$0") 1.1.0         # Release specific version v1.1.0
  $(basename "$0") v2.0.0        # Release specific version v2.0.0
  $(basename "$0") --dry-run     # Dry-run patch bump
EOF
}

# Color formatting
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Check if git is available and in a git repo
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    log_error "Not inside a Git repository."
    exit 1
fi

DRY_RUN=false
INPUT_VERSION=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -*)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
        *)
            if [[ -z "$INPUT_VERSION" ]]; then
                INPUT_VERSION="$1"
            else
                log_error "Unexpected argument: $1"
                show_help
                exit 1
            fi
            shift
            ;;
    esac
done

# Fetch latest tags from remote (if remote exists)
if git remote get-url origin >/dev/null 2>&1; then
    log_info "Fetching tags from remote origin..."
    git fetch --tags origin 2>/dev/null || log_warn "Could not fetch tags from remote origin (continuing with local tags)."
fi

# Get current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$CURRENT_BRANCH" == "HEAD" ]]; then
    log_error "You are in a detached HEAD state. Please checkout a branch before releasing."
    exit 1
fi

# Find the latest semver tag
# Sort tags by version
LATEST_TAG=$(git tag -l "v*" "v*.*.*" "*.*.*" 2>/dev/null | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1 || true)

if [[ -z "$LATEST_TAG" ]]; then
    log_warn "No existing semantic version tags found. Defaulting base to v0.0.0."
    LATEST_TAG="v0.0.0"
fi

log_info "Latest Git tag detected: ${YELLOW}${LATEST_TAG}${NC}"

# Strip leading 'v' for numeric comparisons
LATEST_NUM="${LATEST_TAG#v}"
IFS='.' read -r L_MAJ L_MIN L_PAT <<< "$LATEST_NUM"

# Validate semver helper function
is_valid_semver() {
    local ver="$1"
    [[ "$ver" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

# Compare two semver strings: returns 1 if v1 > v2, 0 if v1 == v2, -1 if v1 < v2
compare_semver() {
    local v1="${1#v}"
    local v2="${2#v}"

    local maj1 min1 pat1 maj2 min2 pat2
    IFS='.' read -r maj1 min1 pat1 <<< "$v1"
    IFS='.' read -r maj2 min2 pat2 <<< "$v2"

    if (( 10#$maj1 > 10#$maj2 )); then echo 1; return; fi
    if (( 10#$maj1 < 10#$maj2 )); then echo -1; return; fi
    if (( 10#$min1 > 10#$min2 )); then echo 1; return; fi
    if (( 10#$min1 < 10#$min2 )); then echo -1; return; fi
    if (( 10#$pat1 > 10#$pat2 )); then echo 1; return; fi
    if (( 10#$pat1 < 10#$pat2 )); then echo -1; return; fi
    echo 0
}

# Determine target version
TARGET_VERSION=""

if [[ -z "$INPUT_VERSION" ]]; then
    # Auto-increment patch version
    NEXT_PAT=$(( 10#$L_PAT + 1 ))
    TARGET_VERSION="v${L_MAJ}.${L_MIN}.${NEXT_PAT}"
    log_info "No version specified. Auto-incrementing patch version to: ${GREEN}${TARGET_VERSION}${NC}"
else
    # Validate specified version format
    if ! is_valid_semver "$INPUT_VERSION"; then
        log_error "Invalid version format '${INPUT_VERSION}'. Expected semver format: X.Y.Z or vX.Y.Z (e.g. 1.0.2 or v1.0.2)"
        exit 1
    fi

    # Ensure 'v' prefix
    if [[ "$INPUT_VERSION" != v* ]]; then
        TARGET_VERSION="v${INPUT_VERSION}"
    else
        TARGET_VERSION="$INPUT_VERSION"
    fi

    # Check that target version is strictly greater than latest tag
    CMP_RES=$(compare_semver "$TARGET_VERSION" "$LATEST_TAG")
    if (( CMP_RES <= 0 )); then
        log_error "Specified version ${TARGET_VERSION} is not greater than the latest tag ${LATEST_TAG}."
        exit 1
    fi

    log_info "Specified version ${GREEN}${TARGET_VERSION}${NC} is valid and greater than ${LATEST_TAG}."
fi

# Check if tag already exists
if git rev-parse "$TARGET_VERSION" >/dev/null 2>&1; then
    log_error "Tag ${TARGET_VERSION} already exists locally."
    exit 1
fi

# Confirm action
echo
echo -e "Release summary:"
echo -e "  - Current branch : ${BLUE}${CURRENT_BRANCH}${NC}"
echo -e "  - Latest tag     : ${YELLOW}${LATEST_TAG}${NC}"
echo -e "  - Target release : ${GREEN}${TARGET_VERSION}${NC}"
echo -e "  - Dry-run mode   : ${DRY_RUN}"
echo

if [[ "$DRY_RUN" == true ]]; then
    log_warn "Dry run enabled. No changes will be committed or pushed."
    exit 0
fi

# Step 1: Check working directory and commit changes if any
STATUS_OUTPUT=$(git status --porcelain)
if [[ -n "$STATUS_OUTPUT" ]]; then
    log_info "Uncommitted changes detected in working directory. Staging and committing..."
    git add -A
    git commit -m "chore(release): release ${TARGET_VERSION}" --trailer "Co-authored-by: Junie <junie@jetbrains.com>"
    log_success "Created release commit for ${TARGET_VERSION}."
else
    log_info "Working directory is clean. No new commit needed before tagging."
fi

# Step 2: Push current branch to remote
log_info "Pushing branch ${CURRENT_BRANCH} to origin..."
git push origin "$CURRENT_BRANCH"
log_success "Branch ${CURRENT_BRANCH} pushed to origin."

# Step 3: Create annotated tag
log_info "Creating annotated tag ${TARGET_VERSION}..."
git tag -a "$TARGET_VERSION" -m "Release ${TARGET_VERSION}"
log_success "Tag ${TARGET_VERSION} created."

# Step 4: Push tag to remote
log_info "Pushing tag ${TARGET_VERSION} to origin..."
git push origin "$TARGET_VERSION"
log_success "Tag ${TARGET_VERSION} pushed to origin."

echo
log_success "Release ${TARGET_VERSION} triggered successfully! 🎉"
log_info "GitHub Actions release workflow is now running."
