WORKSHOP_PATH    := examples/hello-linux
WORKSHOP_IMAGE   := localhost/hello-linux
IMAGES_DIR       := /tmp/workshop-images
BASE_IMAGES_DIR  := /tmp/base-images

.PHONY: test build base-images publish-base-images build-workshop tidy \
	build-feature build-release

test:
	dagger call test --src .

# Cross-compile the single workshop binary (frontend embedded) for linux/amd64
build:
	dagger call build-workshop-binary --src . -o workshop
	chmod +x workshop

# Build all three base images and load them into Podman.
base-images:
	mkdir -p $(BASE_IMAGES_DIR)
	dagger call build-base-images --src . --output $(BASE_IMAGES_DIR)
	@echo "Loading base images into Podman..."
	@for tar in $(BASE_IMAGES_DIR)/*.tar; do \
		variant=$$(basename $$tar .tar); \
		echo "  Loading workshop-base:$$variant"; \
		id=$$(podman load -i $$tar 2>/dev/null | grep -o 'sha256:[a-f0-9]*'); \
		podman tag $$id workshop-base:$$variant; \
	done
	@echo "Done. Base images:"
	@podman images "workshop-base" 2>/dev/null || true

# Build and push all three base images to ghcr.io.
# Requires GITHUB_TOKEN env var with write:packages scope.
publish-base-images:
	dagger call publish-base-images --src . --token env:GITHUB_TOKEN

# Build all step images and load them into Podman as localhost/hello-linux:<step-id>.
# The first step is also tagged :latest so `workshop run localhost/hello-linux` works.
# Note: tarballs are loaded in alphabetical filename order; step IDs must sort
# alphabetically in the intended step order (e.g. step-1-*, step-2-*, step-3-*).
build-workshop:
	mkdir -p $(IMAGES_DIR)
	dagger call build-workshop --src . --workshop-path $(WORKSHOP_PATH) --output $(IMAGES_DIR)
	@echo "Loading images into Podman..."
	@first=1; \
	for tar in $(IMAGES_DIR)/*.tar; do \
		step=$$(basename $$tar .tar); \
		echo "  Loading $(WORKSHOP_IMAGE):$$step"; \
		id=$$(podman load -i $$tar 2>/dev/null | grep -o 'sha256:[a-f0-9]*'); \
		podman tag $$id $(WORKSHOP_IMAGE):$$step; \
		if [ "$$first" = "1" ]; then \
			podman tag $$id $(WORKSHOP_IMAGE):latest; \
			echo "  Tagged $(WORKSHOP_IMAGE):latest → $$step"; \
			first=0; \
		fi; \
	done
	@echo "Done. Images available:"
	@podman images "$(WORKSHOP_IMAGE)" 2>/dev/null || true

# Update go.mod and go.sum via Dagger (no local Go needed)
tidy:
	dagger call go-mod-tidy --src . -o .

# Tarball the devcontainer feature
build-feature:
	dagger call build-feature --src . -o /tmp/workshop-feature.tgz
	@tar tzf /tmp/workshop-feature.tgz

# Build all release assets (both arches, vendored tools, bashrc, installer) —
# the same set CI uploads to GitHub releases
build-release:
	dagger call build-release --src . -o /tmp/release
	@ls -la /tmp/release
