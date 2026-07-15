#!/bin/bash
# =============================================================================
# update-tools.sh - Smart update check for Claude Code and Codex
#
# Runs during container startup (as root, before gosu drops privileges).
# Compares installed versions against npm registry and updates only when
# a newer version is available. Falls back gracefully on network errors.
#
# Controlled by environment variables:
#   AUTO_UPDATE_TOOLS   - "true" (default) to enable, any other value to skip
#   TOOL_UPDATE_TIMEOUT - per-package timeout in seconds (default: 120)
# =============================================================================

set -o pipefail

AUTO_UPDATE="${AUTO_UPDATE_TOOLS:-true}"
TIMEOUT="${TOOL_UPDATE_TIMEOUT:-120}"

if [ "$AUTO_UPDATE" != "true" ]; then
    echo "[update-tools] Auto-update disabled (AUTO_UPDATE_TOOLS=$AUTO_UPDATE)"
    exit 0
fi

updated=0
failed=0
skipped=0

for pkg in @anthropic-ai/claude-code @openai/codex; do
    echo "[update-tools] Checking $pkg ..."

    # Get currently installed version
    current=$(timeout "$TIMEOUT" npm list -g "$pkg" --depth=0 --json 2>/dev/null \
        | grep -o '"version": *"[^"]*"' | head -1 | grep -o '"[^"]*"$' | tr -d '"') || true

    # Get latest version from registry (with timeout)
    latest=$(timeout "$TIMEOUT" npm view "$pkg" version 2>/dev/null) || true

    if [ -z "$latest" ]; then
        echo "[update-tools] WARNING: Could not fetch latest version for $pkg (network issue?). Skipping."
        failed=$((failed + 1))
        continue
    fi

    if [ -z "$current" ]; then
        echo "[update-tools] $pkg not found in image, installing $latest ..."
    elif [ "$current" = "$latest" ]; then
        echo "[update-tools] $pkg is up-to-date ($current)"
        skipped=$((skipped + 1))
        continue
    else
        echo "[update-tools] $pkg update available: $current -> $latest"
    fi

    if timeout "$TIMEOUT" npm install -g "$pkg@latest" >/dev/null 2>&1; then
        echo "[update-tools] $pkg updated to $latest"
        updated=$((updated + 1))
    else
        echo "[update-tools] WARNING: Failed to update $pkg. Build-time version remains available."
        failed=$((failed + 1))
    fi
done

echo "[update-tools] Done: $updated updated, $skipped up-to-date, $failed failed"

# Stamp file for downstream inspection
echo "updated=$updated skipped=$skipped failed=$failed ts=$(date -Iseconds 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)" > /tmp/.tools-updated
