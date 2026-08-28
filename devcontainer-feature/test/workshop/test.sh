#!/bin/bash
# Feature test — run via:
#   devcontainer features test --features workshop \
#     --base-image mcr.microsoft.com/devcontainers/base:ubuntu \
#     --project-folder devcontainer-feature
set -e

echo "Testing workshop feature installation..."

check() {
    if command -v "$1" &>/dev/null; then
        echo "  PASS: $1 found"
    else
        echo "  FAIL: $1 not found"
        exit 1
    fi
}

check workshop
check ttyd
check goss
check workshop-compile-and-setup

if [ -d /workshop/runtime ]; then
    echo "  PASS: /workshop/runtime exists"
else
    echo "  FAIL: /workshop/runtime not found"
    exit 1
fi

if grep -q 'workshop-platform.bashrc' /etc/bash.bashrc; then
    echo "  PASS: bashrc instrumentation installed"
else
    echo "  FAIL: bashrc instrumentation not found in /etc/bash.bashrc"
    exit 1
fi

if [ "$WORKSHOP_MODE" = "devcontainer" ]; then
    echo "  PASS: WORKSHOP_MODE=devcontainer"
else
    echo "  FAIL: WORKSHOP_MODE not set (expected from containerEnv)"
    exit 1
fi

echo "All tests passed."
