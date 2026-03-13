#!/usr/bin/env bash
# Smoke test for built workshop images.
# Usage: ./scripts/smoke-test.sh [image-prefix]
# Default image prefix: localhost/hello-linux
set -euo pipefail

IMAGE="${1:-localhost/hello-linux}"
STEP1="${IMAGE}:step-1-intro"
FAILURES=0

pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1"; FAILURES=$((FAILURES + 1)); }
check() {
    local desc="$1"; shift
    if "$@" &>/dev/null; then pass "$desc"; else fail "$desc"; fi
}

echo "Smoke test: $IMAGE"
echo ""

# ── File presence ─────────────────────────────────────────────────────────────
echo "File presence:"
check "workshop.json exists"                   podman run --rm "$STEP1" test -f /workshop/workshop.json
check "step-1-intro/meta.json exists"          podman run --rm "$STEP1" test -f /workshop/steps/step-1-intro/meta.json
check "step-1-intro/content.md exists"         podman run --rm "$STEP1" test -f /workshop/steps/step-1-intro/content.md
check "step-1-intro/hints.md exists"           podman run --rm "$STEP1" test -f /workshop/steps/step-1-intro/hints.md
check "step-2-files/goss.yaml exists"          podman run --rm "$STEP1" test -f /workshop/steps/step-2-files/goss.yaml
check "step-3-validate/goss.yaml exists"       podman run --rm "$STEP1" test -f /workshop/steps/step-3-validate/goss.yaml
check "workshop-backend binary exists"         podman run --rm "$STEP1" test -x /usr/local/bin/workshop-backend
check "goss binary exists"                     podman run --rm "$STEP1" test -x /usr/local/bin/goss
check "tini binary exists"                     podman run --rm "$STEP1" test -x /sbin/tini

# ── Content checks ────────────────────────────────────────────────────────────
echo ""
echo "Content checks:"
check "workshop.json contains image name"      podman run --rm "$STEP1" grep -q '"image"' /workshop/workshop.json
check "workshop.json has all 3 steps"          podman run --rm "$STEP1" grep -q 'step-3-validate' /workshop/workshop.json
check "step-1-intro/meta.json has hasHints:true" \
    podman run --rm "$STEP1" grep -q '"hasHints": true' /workshop/steps/step-1-intro/meta.json

# ── API endpoints ─────────────────────────────────────────────────────────────
echo ""
echo "API endpoints:"
CID=$(podman run --rm -d -p 18080:8080 "$STEP1")
trap 'podman stop '"$CID"' &>/dev/null || true' EXIT

# Wait for backend to be ready (up to 10s)
for i in $(seq 1 10); do
    curl -sf http://localhost:18080/api/state &>/dev/null && break
    sleep 1
done

state=$(curl -sf http://localhost:18080/api/state || echo "")
check "GET /api/state returns 200"             curl -sf http://localhost:18080/api/state
check "/api/state has activeStep"              bash -c "echo '$state' | grep -q '\"activeStep\"'"
check "/api/state activeStep is step-1-intro"  bash -c "echo '$state' | grep -q '\"activeStep\":\"step-1-intro\"'"

steps=$(curl -sf http://localhost:18080/api/steps || echo "")
check "GET /api/steps returns 200"             curl -sf http://localhost:18080/api/steps
check "/api/steps has step-1-intro"            bash -c "echo '$steps' | grep -q '\"id\":\"step-1-intro\"'"
check "/api/steps has step-2-files"            bash -c "echo '$steps' | grep -q '\"id\":\"step-2-files\"'"

content=$(curl -sf http://localhost:18080/api/steps/step-1-intro/content || echo "")
check "GET /api/steps/step-1-intro/content 200"   curl -sf http://localhost:18080/api/steps/step-1-intro/content
check "/api/steps/step-1-intro/content non-empty" test -n "$content"

check "GET /api/steps/nonexistent/content 404" \
    bash -c '[ "$(curl -o /dev/null -sw "%{http_code}" http://localhost:18080/api/steps/nonexistent/content)" = "404" ]'

podman stop "$CID" &>/dev/null || true
trap - EXIT

# ── Step-2 file mapping check ─────────────────────────────────────────────────
echo ""
echo "Step file mappings:"
STEP2="${IMAGE}:step-2-files"
check "step-2 hello.sh placed at /workspace/hello.sh" \
    podman run --rm "$STEP2" test -f /workspace/hello.sh
check "step-2 hello.sh is executable (mode 0755)" \
    podman run --rm "$STEP2" test -x /workspace/hello.sh

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
if [ "$FAILURES" -eq 0 ]; then
    echo "All tests passed."
else
    echo "$FAILURES test(s) failed."
    exit 1
fi
