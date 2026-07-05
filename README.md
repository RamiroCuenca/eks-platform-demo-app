# eks-platform-demo-app

[![ci](https://github.com/RamiroCuenca/eks-platform-demo-app/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/RamiroCuenca/eks-platform-demo-app/actions/workflows/ci.yml)
[![codeql](https://github.com/RamiroCuenca/eks-platform-demo-app/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/RamiroCuenca/eks-platform-demo-app/actions/workflows/codeql.yml)

A small Go service that exercises the data tier of the EKS platform. It is deliberately
minimal, the platform is the focus, but it is shaped to drive the parts of that platform
that need a real workload: the Aurora PostgreSQL and ElastiCache Redis tiers, the
HPA/Karpenter scaling path, and the KEDA event-driven autoscaler.

This is one of three repositories:

| Repo | Role |
|---|---|
| [`eks-production-platform`](https://github.com/RamiroCuenca/eks-production-platform) | Infrastructure, Terraform/Terragrunt (EKS, networking, data tier, IAM) |
| **`eks-platform-demo-app`** (this repo) | Application source + its build/test/scan/push pipeline |
| `eks-platform-gitops` | Kubernetes manifests, reconciled by ArgoCD |

New images built here are promoted by a CI-opened pull request that updates the image tag
in the gitops repository, merged automatically once that repository's validation gates
pass, so every deployment is an auditable Git event that cleared the same checks as a
human change.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | Liveness, process is up (no data-tier dependency) |
| GET | `/readyz` | Readiness, 503 unless both Aurora and Redis are reachable |
| GET | `/db` | Reads from Aurora over TLS, proves DB connectivity |
| GET | `/cache` | Writes then reads a key in Redis over TLS+AUTH |
| POST | `/enqueue` | Pushes a job onto the Redis work list (the KEDA scale signal) |
| GET | `/metrics` | Prometheus metrics (request rate/latency, jobs processed) |

## Roles

A single binary serves two roles, chosen by `APP_MODE`:

- **`server`** (default), the HTTP API above.
- **`worker`**, drains the Redis work list; this is the deployment KEDA scales on the
  queue's length. It also exposes `/metrics` and `/healthz`.

## Configuration

All configuration comes from the environment. Secret values are read **file-first**: if
`${NAME}_FILE` is set, its contents are used (trimmed); this is how secrets arrive from the
Secrets Store CSI mount on tmpfs. Otherwise `${NAME}` is used.

| Variable | Default | Notes |
|---|---|---|
| `APP_MODE` | `server` | `server` or `worker` |
| `PORT` | `8080` | |
| `DB_HOST` / `DB_PORT` / `DB_NAME` | (none) / `5432` / `appdb` | From the Aurora connection secret |
| `DB_USER` | (none) | Least-privilege application user (not the RDS master) |
| `DB_PASSWORD` / `DB_PASSWORD_FILE` | (none) | Prefer the `_FILE` form |
| `DB_SSLMODE` | `require` | Aurora enforces TLS (`rds.force_ssl=1`) |
| `REDIS_ADDR` | (none) | `host:port` from the Redis connection secret |
| `REDIS_PASSWORD` / `REDIS_PASSWORD_FILE` | (none) | AUTH token; prefer the `_FILE` form |
| `REDIS_TLS` | `true` | In-transit encryption |
| `REDIS_QUEUE_KEY` | `demo:jobs` | Work-list key shared by `/enqueue` and the worker |

## Develop

```sh
make test      # go test -race ./...
make vet       # go vet ./...
make build     # go build ./...
make run       # run the server locally (set DB_*/REDIS_* first)
make docker    # build the container image
```

## Container

Multi-stage build → `gcr.io/distroless/static-debian12:nonroot`: a static, non-root,
shell-less final image. The image is scanned by Trivy (HIGH/CRITICAL fail the build) and the
Go source by CodeQL on every pull request.

## Load tests

[k6](https://k6.io/) scripts under `loadtest/` drive the two autoscaling paths:

```sh
k6 run loadtest/http.js      # CPU/HTTP load → HPA + Karpenter
k6 run loadtest/enqueue.js   # queue backlog → KEDA Redis scaler
```
