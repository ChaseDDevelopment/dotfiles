#!/usr/bin/env bash
# =============================================================================
# release.sh — Tag and push a new release
# =============================================================================
# Usage: ./release.sh [patch|minor|major]   (default: patch)
#
# Looks at the latest git tag, bumps the version, tags, and pushes.
# CI handles the rest (builds binaries, creates GitHub Release).
set -euo pipefail

bump="${1:-patch}"

# Validate argument
case "$bump" in
    patch|minor|major) ;;
    *)
        echo "Usage: ./release.sh [patch|minor|major]"
        exit 1
        ;;
esac

# Get latest tag
latest=$(git tag --sort=-v:refname | head -1)
if [[ -z "$latest" ]]; then
    echo "No existing tags found. Starting at v0.0.1"
    next="v0.0.1"
else
    # Strip leading 'v' and split
    version="${latest#v}"
    IFS='.' read -r major minor patch_num <<< "$version"

    case "$bump" in
        major) major=$((major + 1)); minor=0; patch_num=0 ;;
        minor) minor=$((minor + 1)); patch_num=0 ;;
        patch) patch_num=$((patch_num + 1)) ;;
    esac

    next="v${major}.${minor}.${patch_num}"
fi

echo "Current: ${latest:-none}"
echo "Next:    $next"
echo ""
read -rp "Tag and push $next? [y/N] " confirm
if [[ "$confirm" != [yY] ]]; then
    echo "Aborted."
    exit 0
fi

git tag "$next"
git push && git push origin "$next"

echo ""
echo "Released $next — CI will build and publish."
