#!/bin/bash
# DevContainer feature install script — runs as root at container build time.
#
# Downloads all platform binaries from a single GitHub release (one version
# tag pins everything — no version constants here) and bakes the feature
# options into the workshop-compile-and-setup helper, since option env vars
# (WORKSHOPPATH, STEP) exist only during this script, not at runtime.
set -e

VERSION="${VERSION:-latest}"
WORKSHOP_PATH="${WORKSHOPPATH:-.}"
START_STEP="${STEP:-}"

REPO="asocpro/workshop-builder"
GITHUB="https://github.com/${REPO}"

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Resolve "latest" to an actual tag
if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi

echo "Installing workshop platform tools ${VERSION} (${ARCH})..."

# All binaries (ours + vendored ttyd/goss) come from the GitHub release.
for BINARY in workshop ttyd goss; do
    URL="${GITHUB}/releases/download/${VERSION}/${BINARY}-linux-${ARCH}"
    echo "  Downloading ${BINARY}..."
    curl -fsSL "$URL" -o "/usr/local/bin/${BINARY}"
    chmod +x "/usr/local/bin/${BINARY}"
done

# Command logging instrumentation from the release.
# Single source of truth: backend/instrumentation/workshop-platform.bashrc.
curl -fsSL "${GITHUB}/releases/download/${VERSION}/workshop-platform.bashrc" \
    -o /etc/workshop-platform.bashrc

# Source from the system bash profile (same pattern as the base image build).
# Global sourcing is fine here — the container exists only for the workshop.
if ! grep -q 'workshop-platform.bashrc' /etc/bash.bashrc 2>/dev/null; then
    echo 'source /etc/workshop-platform.bashrc' >> /etc/bash.bashrc
fi

# Runtime directory with open permissions (postCreateCommand runs as the
# container user, not root)
mkdir -p /workshop/runtime
chmod 777 /workshop /workshop/runtime

# Generate the compile-and-setup helper with the feature options baked in —
# a thin wrapper, all logic lives in the Go binaries. postCreateCommand runs
# in the workspace folder, so a relative workshopPath resolves correctly.
cat > /usr/local/bin/workshop-compile-and-setup << HELPER
#!/bin/bash
set -e

workshop compile "${WORKSHOP_PATH}" --output-dir /workshop

if [ -n "${START_STEP}" ]; then
    workshop setup --workshop-dir /workshop --step "${START_STEP}"
fi

echo "Workshop compiled and ready — UI at http://localhost:8080 once the backend starts."
HELPER
chmod +x /usr/local/bin/workshop-compile-and-setup

echo "Workshop platform tools installed."
