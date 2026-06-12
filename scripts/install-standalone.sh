#!/bin/sh
# Installs the workshop platform tools for standalone mode onto a Linux server.
#
#   curl -fsSL https://github.com/asocpro/workshop-builder/releases/latest/download/install-standalone.sh | sh
#
# Unlike the devcontainer feature's install.sh, this script:
#   - does NOT touch /etc/bash.bashrc or install a global bashrc — the backend
#     embeds the instrumentation and scopes it to workshop terminal sessions
#   - does NOT create /workshop — `workshop-backend --serve` resolves its own
#     root under ~/.workshop/<name>
#
# The script's only job is fetching binaries. Everything else the binary sets
# up on its own. Set WITH_COMPILE=1 to also install compile-workshop (for
# pre-compiling or CI validation; --serve embeds compilation).
set -eu

VERSION="${VERSION:-latest}"
PREFIX="${PREFIX:-/usr/local/bin}"
REPO="asocpro/workshop-builder"
GITHUB="https://github.com/${REPO}"

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [ "$(uname -s)" != "Linux" ]; then
    echo "Standalone mode is Linux-only (ttyd and goss have no other builds)." >&2
    exit 1
fi

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
fi

BINARIES="workshop-backend workshop-setup ttyd goss"
if [ "${WITH_COMPILE:-0}" = "1" ]; then
    BINARIES="$BINARIES compile-workshop"
fi

echo "Installing workshop standalone tools ${VERSION} (${ARCH}) to ${PREFIX}..."
for BINARY in $BINARIES; do
    URL="${GITHUB}/releases/download/${VERSION}/${BINARY}-linux-${ARCH}"
    echo "  ${BINARY}"
    curl -fsSL "$URL" -o "${PREFIX}/${BINARY}"
    chmod +x "${PREFIX}/${BINARY}"
done

cat <<'EOF'

Installed. To serve a workshop from a checkout:

    workshop-backend --serve ./my-workshop
    # then open http://localhost:8080 (or SSH port-forward from your machine)

To run persistently as a systemd service:

    workshop-backend service install --serve /opt/my-workshop \
        --listen 0.0.0.0:8080 --auth-user admin --auth-password-file /etc/workshop/pass

Private networks only — see docs/platform/standalone-mode.md.
EOF
