# Base Images — Platform Foundation Layers

## Purpose

Maintained platform images with all workshop tooling pre-installed. Authors `FROM workshop-base:<distro>` and layer on their workshop content. Base images provide OCI layer deduplication across workshops, consistent tooling versions, and zero-config platform setup.

## Available Base Images

| Image | Distro | Use Case |
|---|---|---|
| `workshop-base:alpine` | Alpine Linux | Lightweight workshops, minimal footprint |
| `workshop-base:ubuntu` | Ubuntu LTS | General Linux workshops, broadest package availability |
| `workshop-base:centos` | CentOS Stream | RHEL-ecosystem workshops, enterprise tooling |

## What's Included

Every base image contains:

| Component | Path | Purpose |
|---|---|---|
| tini | `/sbin/tini` | PID 1 init — zombie reaping, signal forwarding |
| workshop-backend | `/usr/local/bin/workshop-backend` | Runtime engine — web UI, API, terminal proxy, recording |
| goss | `/usr/local/bin/goss` | Step validation — tests student's container state |
| asciinema | `/usr/bin/asciinema` | Terminal recording — full session capture with replay |
| workshop-platform.bashrc | `/etc/workshop-platform.bashrc` | Shell instrumentation — PROMPT_COMMAND for command logging |
| runtime directory | `/workshop/runtime/` | Pre-created in image; populated at runtime with JSONL logs, recording, state events |

The entrypoint is pre-configured:

```
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

## Shell Instrumentation

Every base image sources `/etc/workshop-platform.bashrc` which installs a `PROMPT_COMMAND` hook for structured command logging. See [Instrumentation](./instrumentation.md) for the full implementation.

The bashrc is sourced automatically for interactive bash sessions. It does not interfere with non-interactive shells or scripts.

## Image Layer Structure

```
workshop-base:ubuntu
  ├── Ubuntu LTS base layers
  ├── Platform tooling layer:
  │   ├── tini
  │   ├── workshop-backend (with embedded web UI assets)
  │   ├── goss
  │   ├── asciinema
  │   └── /etc/workshop-platform.bashrc
  └── ENTRYPOINT configuration
         │
         ▼ Author layers (via workshop.yaml build)
myorg/kubernetes-101:step-1-intro
  ├── apt-get install kubectl helm ...     (author's packages)
  ├── /workshop/workshop.json              (workshop identity + step list)
  ├── /workshop/steps/step-pods/           (ALL steps' metadata)
  │   ├── meta.json
  │   ├── content.md
  │   ├── goss.yaml
  │   └── llm.json
  ├── /workshop/steps/step-services/
  │   └── ...
  └── /workspace/...                       (step-specific content files)
```

## OCI Layer Deduplication

Multiple workshops built on the same base image share base layers in the registry:

```
workshop-base:ubuntu (shared layers — pulled once)
  │
  ├── myorg/kubernetes-101:step-1-intro  (unique content layers only)
  ├── myorg/kubernetes-101:step-2-deploy (unique content layers only)
  ├── myorg/docker-basics:step-1         (unique content layers only)
  └── myorg/linux-admin:step-1           (unique content layers only)
```

Students pulling multiple workshops on the same base download the base layers once. This significantly reduces bandwidth and storage for workshop events with multiple workshops.

## Platform Updates

Updating platform tooling (new backend version, security patches) is a base image rebuild:

1. Rebuild `workshop-base:{alpine,ubuntu,centos}` with updated components
2. Authors rebuild their workshops: `workshop build compile` (picks up new base layers)
3. No changes to `workshop.yaml` required

The base image tag (e.g. `workshop-base:ubuntu`) is a rolling tag pointing to the latest platform release. For pinned builds, use digest references or versioned tags (e.g. `workshop-base:ubuntu-v1.2.0`).

## Using a Custom Base Image

Authors who need a non-standard base can use `base.image` or `base.containerFile` in their `workshop.yaml`:

```yaml
base:
  image: golang:1.22
```

or:

```yaml
base:
  containerFile: ./Containerfile.base
```

When using a custom base image, the author is responsible for installing all required platform components. The Dagger pipeline does **not** attempt to inject the platform layer — different distros have different package managers, library paths, and shell configurations, making automatic injection unreliable.

The tradeoff: custom base images don't benefit from OCI layer deduplication with other workshops. Each workshop has its own unique base layers.

### Custom Base Image Requirements

The following components must be present in the custom base image for the workshop platform to function. All binaries are available as static downloads from the platform release artifacts.

| Component | Required Path | Purpose | Notes |
|---|---|---|---|
| tini | `/sbin/tini` | PID 1 init — zombie reaping, signal forwarding | Static binary; download from [tini releases](https://github.com/krallin/tini/releases) |
| workshop-backend | `/usr/local/bin/workshop-backend` | Runtime engine — web UI, API, terminal proxy | Static Go binary with embedded assets; provided by platform releases |
| goss | `/usr/local/bin/goss` | Step validation | Static binary; download from [goss releases](https://github.com/goss-org/goss/releases). Only required if any step uses `goss` or `gossFile`. |
| asciinema | `/usr/bin/asciinema` | Terminal session recording | Python package or static build. Only required if session recording is enabled. |
| workshop-platform.bashrc | `/etc/workshop-platform.bashrc` | Shell instrumentation for command logging | Plain shell script; provided by platform releases |

Additionally, the custom base image must:

1. **Source the shell instrumentation** — add to `/etc/bash.bashrc` (or equivalent):
   ```bash
   [ -f /etc/workshop-platform.bashrc ] && . /etc/workshop-platform.bashrc
   ```
2. **Set the entrypoint**:
   ```dockerfile
   ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
   ```
3. **Set the working directory**:
   ```dockerfile
   WORKDIR /workspace
   ```
4. **Have bash installed** — the terminal proxy and shell instrumentation require bash.

### Validation

The `workshop build compile` command validates that the required components exist in the base image before building steps. If a required binary is missing, the build fails with a clear error message indicating which component is absent and where to obtain it.

## Building Base Images

Base images are built from Containerfiles maintained by the platform team:

```dockerfile
# workshop-base:ubuntu
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Platform tooling
COPY tini /sbin/tini
COPY workshop-backend /usr/local/bin/workshop-backend
COPY goss /usr/local/bin/goss
COPY asciinema /usr/bin/asciinema
COPY workshop-platform.bashrc /etc/workshop-platform.bashrc

RUN chmod +x /sbin/tini /usr/local/bin/workshop-backend /usr/local/bin/goss /usr/bin/asciinema

# Source shell instrumentation for interactive bash sessions
RUN echo '[ -f /etc/workshop-platform.bashrc ] && . /etc/workshop-platform.bashrc' >> /etc/bash.bashrc

# Runtime directory created by backend on startup
RUN mkdir -p /workshop/runtime

WORKDIR /workspace

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/workshop-backend"]
```

## Relationship to Other Components

| Component | Relationship |
|---|---|
| [Dagger Pipeline](../artifact/compilation.md) | Builds workshop images on top of base images; validates platform components for custom bases |
| [Backend Service](./backend-service.md) | Pre-installed in base images; the runtime engine |
| [Instrumentation](./instrumentation.md) | Shell bashrc pre-installed; enables command logging |
| [Workshop Spec](../definition/workshop.md) | `base.image` field references a base image |
