WORKSHOP_PATH  := examples/hello-linux
WORKSHOP_IMAGE := localhost/hello-linux
IMAGES_DIR     := /tmp/workshop-images

.PHONY: test build-backend build-workshop

test:
	dagger call test --src .

build-backend:
	dagger call build-backend --src .

# Build all step images and load them into Podman as localhost/hello-linux:<step-id>
build-workshop:
	mkdir -p $(IMAGES_DIR)
	dagger call build-workshop --src . --workshop-path $(WORKSHOP_PATH) --output $(IMAGES_DIR)
	@echo "Loading images into Podman..."
	@for tar in $(IMAGES_DIR)/*.tar; do \
		step=$$(basename $$tar .tar); \
		echo "  Loading $(WORKSHOP_IMAGE):$$step"; \
		id=$$(podman load -i $$tar 2>/dev/null | grep -o 'sha256:[a-f0-9]*'); \
		podman tag $$id $(WORKSHOP_IMAGE):$$step; \
	done
	@echo "Done. Images available:"
	@podman images "$(WORKSHOP_IMAGE)" 2>/dev/null || true
