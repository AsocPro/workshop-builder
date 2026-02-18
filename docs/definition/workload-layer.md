# Workload Layer — `docker-compose.yml`

## Purpose

Defines container topology. This is purely a workload description — it describes **what containers run**, not how they are managed, isolated, or lifecycled.

## What It Contains

- `services`
- `image`
- `ports`
- `volumes`
- `environment`
- `command`
- Minimal `depends_on`

## What It Does NOT Contain

- TTL
- Lifecycle mode (persistent/ephemeral)
- Team isolation
- Namespace semantics
- RBAC
- Cluster provisioning config
- Quotas
- Resource classes

These concerns belong in [`workspace.yaml`](./workspace-metadata.md).

## Design Rationale

Compose is an industry-standard format for describing container topology. By limiting it to this role:

- Workshop authors use familiar tooling
- No custom extensions or x-fields are required for platform semantics
- The same Compose file works in local Docker mode and cluster mode
- Compose files remain portable and testable outside the platform

## Consumers

The raw `docker-compose.yml` is consumed only at compile time. All runtime consumers read exclusively from the [SQLite artifact](../artifact/sqlite-artifact.md), which contains the per-step environment metadata derived during compilation.

| Consumer | Source | How It Uses It |
|---|---|---|
| Build system (compiler) | Raw `docker-compose.yml` | Only consumer of Compose directly; parses, validates, and translates into SQLite artifact |
| CLI (local Docker mode) | SQLite artifact | Reads per-step container metadata; drives `docker run` lifecycle for step transitions |
| [Operator](../platform/operator.md) (cluster mode) | SQLite artifact | Reads per-step K8s manifests; applies to workload namespace for step transitions |

Step transitions in all modes are driven by what the SQLite artifact says for that step — not by re-reading or re-interpreting the original Compose file.

## Supported Compose Subset

The platform supports a minimal, stable subset of Compose focused on container topology only.

### Supported Fields

| Field | Notes |
|---|---|
| `services` | Top-level service map |
| `image` | Required per service; must be a registry reference (no local builds) |
| `ports` | Host/container port mappings; translated to K8s Service objects |
| `volumes` | Named volume mounts; translated to PVCs in cluster mode |
| `environment` | Key/value env vars |
| `command` | Entrypoint override |
| `depends_on` | Simple list form only (`depends_on: [db]`); used for startup ordering |

### Explicitly Excluded

| Field | Reason |
|---|---|
| `build` | Requires image build pipeline and registry push; deferred to future release (see below) |
| `networks` | Unnecessary — pods within a namespace have flat networking in K8s |
| `deploy` | Resource/replica config belongs in `workspace.yaml`, not Compose |
| `secrets` / `configs` | Non-trivial K8s mapping; out of scope for v1 |
| `healthcheck` | Maps to readiness/liveness probes; additive in a future release |
| `depends_on` condition variants | Only simple list form supported; `service_healthy` etc. are out of scope |

### Note on `build:`

`build:` support is a planned future feature. The intent is to make authoring easy for instructors who want to build from a Dockerfile they've written. It is deferred because it requires an image build pipeline, registry credentials, image tagging conventions, and cache invalidation logic — a meaningful addition that is not needed for v1.

For now, authors who need a custom image should build and push it themselves and reference it via `image:`.

## Translation Rules

Each Compose service is translated to a set of Kubernetes objects by the shared library during compilation. The results are stored in the SQLite artifact — the operator and CLI never perform translation at runtime.

| Compose | Kubernetes Object | Notes |
|---|---|---|
| `services.<name>` | `Deployment` | One Deployment per service; single replica |
| `services.<name>` | `Service` | ClusterIP Service per service; preserves the service name as a stable hostname (see note below) |
| `ports` | `Service` (NodePort or LoadBalancer) | Only when ports need external exposure; type determined by `workspace.yaml` access config |
| `volumes` | `PersistentVolumeClaim` | One PVC per named volume; access mode `ReadWriteOnce` |
| `environment` | `Deployment.spec.template.spec.containers[].env` | Inlined as env vars directly on the container spec |
| `command` | `Deployment.spec.template.spec.containers[].args` | Overrides default image command |
| `depends_on` | Init container ordering | Services listed as dependencies are waited on via a simple init container readiness check |

### Service Names as Hostnames

In Docker Compose, containers reach each other using the service name as a hostname — for example, an `app` service connects to a `db` service at `db:5432`. This works the same way in cluster mode.

For every Compose service, the platform creates a Kubernetes Service object with the same name. This gives every service a stable DNS entry inside the workspace. Containers connect to each other using the exact same hostnames they would in Compose — no changes to connection strings or environment variables are needed between local and cluster mode.

Authors do not need to know how Kubernetes networking works. If it works in Compose, the hostnames will work in cluster mode.

### Local Docker Mode Translation

In local Docker mode the SQLite artifact stores `docker run` parameters rather than K8s manifests. The mapping is straightforward:

| Compose | docker run equivalent |
|---|---|
| `image` | image argument |
| `ports` | `-p` flags |
| `volumes` | `-v` flags |
| `environment` | `-e` flags |
| `command` | command argument |
| `depends_on` | start order enforced by the CLI before launching dependent containers |

## Validation

The [Shared Go Library](../platform/shared-go-library.md) validates Compose files against the supported subset before any execution or translation occurs. Validation errors are returned as structured errors with the field path and a human-readable message.

| Rule | Error Message |
|---|---|
| `image` is missing on a service | `service "<name>": image is required` |
| `image` uses a local build reference (`.` or a path) | `service "<name>": image must be a registry reference; local builds are not supported (use build and push first)` |
| An unsupported top-level field is present (e.g. `networks`, `secrets`) | `unsupported field "<field>": not part of the supported Compose subset` |
| An unsupported service-level field is present (e.g. `deploy`, `healthcheck`, `build`) | `service "<name>": unsupported field "<field>"` |
| `depends_on` uses condition form | `service "<name>": depends_on condition variants are not supported; use simple list form` |
| `depends_on` references a service not defined in the file | `service "<name>": depends_on references unknown service "<dep>"` |
| A volume is referenced in a service but not declared in the top-level `volumes` map | `service "<name>": volume "<vol>" is not declared in the top-level volumes map` |

## Examples

### Single Container

The minimal case — one service, no dependencies.

```yaml
services:
  app:
    image: nginx:latest
    ports:
      - "8080:80"
```

### Application + Database

The most common workshop pattern — a primary service with a database dependency.

```yaml
services:
  app:
    image: myorg/workshop-app:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://user:pass@db:5432/workshop
    depends_on:
      - db

  db:
    image: postgres:15
    environment:
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: workshop
    volumes:
      - db-data:/var/lib/postgresql/data

volumes:
  db-data:
```

### With Persistent Volume and Command Override

```yaml
services:
  app:
    image: myorg/workshop-app:latest
    ports:
      - "8080:8080"
    volumes:
      - app-data:/data
    command: ["./server", "--data-dir", "/data"]

volumes:
  app-data:
```
