<!-- Last reviewed against commit 4a51902. -->
# Integrations & environment reference

This is the **single home** for environment variables and external services; other docs link here.

## External services
| Service | Used by | How it's wired | Refs |
|---------|---------|----------------|------|
| **Google / OIDC provider** | `internal/auth` | OAuth2 auth-code + PKCE; `OIDC_*` env; Google options `AccessTypeOffline` + `prompt=consent`. | `internal/auth/browser.go:42-63`, `service.go:32-46` |
| **Postgres (release)** | API, migrate/setup Jobs | CloudNativePG `Cluster` `crew-demo-postgres`; reached at `crew-demo-postgres-rw`; creds from generated secrets. | `deploy/infra/postgres-cluster.yaml` |
| **Postgres (preview)** | same | Plain `postgres:18` Deployment (emptyDir, `trust` auth) — disposable. | `deploy/preview-infra/postgres-deployment.yaml` |
| **OpenTelemetry collector** | framework `httpapp` telemetry | OTLP/HTTP to the namespace `otel-collector:4318` (framework default); only `OTEL_SERVICE_NAME` is set in-cluster. | `deploy/app/base/api-deployment.yaml:88-89` |
| **Gateway component** | edge | `ghcr.io/adiom-data/components/gateway` image, configured by `gateway.json`; validates tokens, serves SPA, forwards verified bearer. | `services/gateway/`, `MODULE.bazel:56-62` |
| **Container registry (ghcr.io/adiom-data)** | build/deploy | OCI push of `crew-demo-app-{api,gateway,migrate}`; base images pulled via `oci.pull`. | `cmd/*/BUILD.bazel`, `deploy/BUILD.bazel`, `MODULE.bazel:49-78` |

Pinned base images (`MODULE.bazel`): `gcr.io/distroless/static-debian12` (api/migrate),
`ghcr.io/adiom-data/components/gateway:v0.0.1`, `ghcr.io/adiom-data/components/goosemigrate:v0.0.1`.

## Environment variables
Consumed by `cmd/api/main.go` (the `environment` struct) unless noted. `Source` = where the value
comes from in the release manifests.

### API — database
| Var | Default | Required | Source |
|-----|---------|----------|--------|
| `PGHOST` | — | yes | hardcoded `crew-demo-postgres-rw` (`api-deployment.yaml`) |
| `PGPORT` | `5432` | no | — |
| `PGDATABASE` | `postgres` | no | `app` (`api-deployment.yaml`) |
| `PGUSER` | `postgres` | no | secret `crew-demo-postgres-app:username` |
| `PGPASSWORD` | — | yes | secret `crew-demo-postgres-app:password` |
| `PGSSLMODE` | `disable` | no | — |

### API — auth / token issuer
| Var | Default | Required | Source / meaning |
|-----|---------|----------|------------------|
| `AUTH_ISSUER` | — | **yes** | `http://crew-demo-api/auth` — issuer URL reachable by token verifiers (gateway + API). |
| `AUTH_KEY_ID` | `sample-auth-2026-06` | no | signing key id. |
| `AUTH_PRIVATE_KEY_BASE64` | — | **yes** | secret `crew-demo-auth` — base64 32-byte Ed25519 seed; **stable across replicas**. |
| `AUTH_STATE_KEY_BASE64` | — | **yes** | secret `crew-demo-auth` — base64 32-byte OAuth-state seed; **stable across replicas**. |
| `AUTH_INSECURE_COOKIES` | `false` | no | `true` for local http dev only. |
| `PUBLIC_BASE_URL` | — | no | base for `/auth/callback` redirect when set. |
| `PROXY_REDIRECT_URL` | — | no | secret `crew-demo-auth` (optional) — stable preview proxy callback. |

### API — OIDC provider
| Var | Default | Required | Source |
|-----|---------|----------|--------|
| `OIDC_ISSUER` | — | **yes** | secret `crew-demo-auth` (e.g. `https://accounts.google.com`). |
| `OIDC_CLIENT_ID` | — | **yes** | secret `crew-demo-auth`. |
| `OIDC_CLIENT_SECRET` | — | **yes** | secret `crew-demo-auth`. |
| `OIDC_ALLOWED_AUDIENCES` | — | no | secret `crew-demo-auth` (comma-separated; the client id is always allowed). |

### Framework (`httpapp` / telemetry)
| Var | Default | Notes |
|-----|---------|-------|
| `PORT` | `:8080` (DefaultAddr) | listen port; framework prepends `:` (set `PORT=8080`, not `:8080`). |
| `OTEL_SERVICE_NAME` | executable name | set to `crew-demo-api` in-cluster. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` and other `OTEL_*` | framework defaults | not set in-cluster; telemetry is effectively off unless an endpoint is provided. |

### `cmd/seed` (env struct)
`PGHOST` (default `localhost`), `PGPORT` (`5432`), `PGDATABASE` (`app`), `PGUSER` (`postgres`),
`PGPASSWORD`, `PGSSLMODE` (`disable`).

### Migration Job (`deploy/migrations/migration-job.yaml`)
`DB_HOST` (`crew-demo-postgres-rw`), `DB_NAME` (`app`), `DB_USER`/`DB_PASSWORD` (secret
`crew-demo-postgres-app`).

### Seed Job (`deploy/seed/seed-job.yaml`, preview only)
`PGHOST` (`crew-demo-postgres-rw`), `PGDATABASE` (`app`), `PGSSLMODE` (`disable`), `PGUSER`/`PGPASSWORD`
(secret `crew-demo-postgres-app`). Same `PG*` scheme as `cmd/seed`; runs the `crew-demo-app-seed` image
after migrations to insert demo partners. Preview only — not in the release deploy.

### Setup Job (`deploy/migrations/setup-job.yaml`)
`PGHOST` (`crew-demo-postgres-rw`), `PGDATABASE` (`postgres`), `PGUSER`/`PGPASSWORD` (secret
`crew-demo-postgres-superuser`), `APP_DATABASE` (`app`), `APP_DATABASE_OWNER` (secret
`crew-demo-postgres-app:username`), `DB_SETUP_WAIT_SECONDS` (`900`).

### Gateway
`GATEWAY_CONFIG` = `/etc/adiom/gateway.json` (`services/gateway/BUILD.bazel`).

## Kubernetes secrets (created by the environment, not checked in)
| Secret | Keys | Consumed by |
|--------|------|-------------|
| `crew-demo-auth` | `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `AUTH_PRIVATE_KEY_BASE64`, `AUTH_STATE_KEY_BASE64`; optional `OIDC_ALLOWED_AUDIENCES`, `PROXY_REDIRECT_URL` | API Deployment |
| `crew-demo-postgres-app` | `username`, `password` | API, migration Job, setup Job |
| `crew-demo-postgres-superuser` | `username`, `password` | setup Job |

In **release**, the Postgres secrets are generated by CloudNativePG. In **preview**, the environment
must supply them (e.g. `username: app`/`postgres`, empty password). Generate the two auth seeds with
`openssl rand -base64 32` and keep them stable — see [invariants.md](invariants.md) INV-1/INV-2.

For the Google OAuth web client, set the authorized redirect URI to `https://<host>/auth/callback`
(release), `http://localhost:5173/auth/callback` (local dev), or `<PROXY_REDIRECT_URL>` (preview).
