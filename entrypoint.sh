#!/bin/bash
set -e

# =============================================================================
# Container Entrypoint
# Handles: UID mapping, JDK switching, env loading
# =============================================================================

# Ensure /usr/local/bin is in PATH
export PATH="/usr/local/bin:$PATH"

# ---------------------------------------------------------------------------
# Load environment files
# ---------------------------------------------------------------------------
load_env_file() {
    local file="$1"
    if [ -f "$file" ]; then
        set -a
        # shellcheck source=/dev/null
        source "$file"
        set +a
    fi
}

# Load global agent env
load_env_file "/home/node/.agents-hub/agents/.env"

# Load per-agent env if AGENT_ID is set
if [ -n "${AGENT_ID:-}" ]; then
    load_env_file "/home/node/.agents-hub/agents/${AGENT_ID}/.env"
fi

# ---------------------------------------------------------------------------
# Initialize SDKMAN
# ---------------------------------------------------------------------------
init_sdkman() {
    if [ -f "/usr/local/sdkman/bin/sdkman-init.sh" ]; then
        export SDKMAN_DIR="/usr/local/sdkman"
        # shellcheck source=/dev/null
        source "/usr/local/sdkman/bin/sdkman-init.sh"
    fi
}

# ---------------------------------------------------------------------------
# JDK Version Mapping
# ---------------------------------------------------------------------------
declare -A JDK_VERSIONS=(
    [8]="8.0.442-tem"
    [11]="11.0.26-tem"
    [17]="17.0.14-tem"
    [21]="21.0.6-tem"
    [25]="25.0.0-tem"
)

# ---------------------------------------------------------------------------
# Switch JDK version
# ---------------------------------------------------------------------------
switch_jdk() {
    local version="${1:-}"

    if [ -z "$version" ]; then
        return 0
    fi

    local full_version="${JDK_VERSIONS[$version]:-}"

    if [ -z "$full_version" ]; then
        echo "Warning: Unknown JDK version '${version}'. Available: ${!JDK_VERSIONS[*]}" >&2
        return 1
    fi

    if ! command -v sdk &> /dev/null; then
        echo "Warning: SDKMAN not available, cannot switch JDK" >&2
        return 1
    fi

    if sdk use java "$full_version" 2>/dev/null; then
        echo "Switched to JDK ${version} (${full_version})"
    else
        echo "Warning: JDK ${full_version} not installed" >&2
        return 1
    fi
}

# ---------------------------------------------------------------------------
# Map container user UID/GID to match host
# ---------------------------------------------------------------------------
map_user_ids() {
    if [ "$(id -u)" != "0" ] || [ -z "${HOST_UID:-}" ]; then
        return 0
    fi

    # Guard: HOST_UID=0 would make node user root, defeating gosu.
    # Fall back to default UID 1000 to ensure privilege drop works.
    if [ "${HOST_UID}" = "0" ]; then
        echo "[entrypoint] Warning: HOST_UID=0 detected, falling back to UID 1000 for node user" >&2
        HOST_UID=1000
        HOST_GID=${HOST_GID:-1000}
    fi

    local current_uid
    current_uid=$(id -u node)

    if [ "$current_uid" != "${HOST_UID}" ]; then
        local old_uid
        old_uid=$(id -u node)

        groupmod -g "${HOST_GID}" node 2>/dev/null || true
        usermod -u "${HOST_UID}" -g "${HOST_GID}" node

        # Fix files created during build with old UID
        find /home/node -user "${old_uid}" -exec chown -h node:node {} + 2>/dev/null || true
    fi

    # Always drop privileges to node user via gosu, even when UID already
    # matches. Without this, the container runs as root when HOST_UID=1000.
    exec gosu node "$@"
}

# ---------------------------------------------------------------------------
# Auto-update AI tools (Claude Code, Codex)
# Runs as root before gosu drops privileges.
# Controlled by AUTO_UPDATE_TOOLS env var (default: true).
# ---------------------------------------------------------------------------
update_ai_tools() {
    if [ -x /usr/local/bin/update-tools.sh ]; then
        /usr/local/bin/update-tools.sh || echo "[entrypoint] Tool update failed, continuing with build-time versions"
        # Fix npm cache ownership: npm install -g runs as root and may write
        # to /home/node/.npm/ with root ownership. chown ensures the node
        # user can access the cache after gosu drops privileges.
        chown -R node:node /home/node/.npm 2>/dev/null || true
    fi
}

# =============================================================================
# Main
# =============================================================================

init_sdkman
switch_jdk "${JDK_VERSION:-}"
update_ai_tools
map_user_ids "$@"

exec "$@"
