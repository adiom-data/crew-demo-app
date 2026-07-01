# crew-demo-app — On-board

**On-board** is a small, production-shaped **partner onboarding & management portal** — the demo
canvas for Adiom Crew. An ops/CS admin signs in with Google, manages partners on a dashboard, and
partners can self-register through a public form. It is a Bazel-only monorepo: a Go + Connect-RPC
API/auth server, a Postgres store, and a Vite/React SPA, served in production behind a component
gateway and delivered to Kubernetes via FluxCD. Forked from `adiom-data/sample-app`.

```
                    ┌──────────────────────────────────────────────┐
  Browser ──HTTPS──▶│ Gateway (validates app token, serves SPA)     │
                    │  public: /auth/*, /adiom.auth.v1.*,           │
                    │          /sample.v1.OnboardingService/*       │
                    │  protected: /sample.v1.SampleService/*,       │
                    │             /sample.v1.PartnerService/*        │
                    └───────────────┬──────────────────────────────┘
                        verified bearer │  (forwarded)
                                        ▼
                    ┌──────────────────────────────────────────────┐
                    │ API (Go + Connect)   ── OIDC ──▶ Google        │
                    │  internal/api  (Sample/Partner/Onboarding)     │
                    │  internal/auth (BFF /auth, token issuer)       │
                    └───────────────┬──────────────────────────────┘
                                    ▼
                            Postgres (goose)
                     app_users · auth_sessions · partners · activities
```

## Quick start (local)
Run the whole stack locally with real Google OIDC (full runbook in
[docs/operations.md](docs/operations.md#local-development)):

```sh
# 1. Postgres
docker run -d --name crew-demo-pg -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=app \
  -p 55432:5432 postgres:18

# 2. Migrate + seed
goose -dir services/api/migrations postgres \
  "postgres://postgres:pass@localhost:55432/app?sslmode=disable" up
PGHOST=localhost PGPORT=55432 PGDATABASE=app PGUSER=postgres PGPASSWORD=pass \
  bazel run //cmd/seed

# 3. API (needs a Google OIDC web client; redirect URI http://localhost:5173/auth/callback)
#    plus AUTH_* keys (openssl rand -base64 32). See docs/operations.md for the full env.
go run ./cmd/api

# 4. SPA (proxies /auth and Connect calls to the API on :8080)
cd web && pnpm install && pnpm dev   # http://localhost:5173
```

The public **`/onboard`** form works without login; the dashboard requires Google sign-in.

## Common commands
```sh
bazel build //...                     # build everything (Go, web bundle, images)
bazel test  //...                     # Go unit tests + Vitest (//web:test)
buf generate                          # regenerate proto stubs (gen/go/**, web/src/gen/**)
bazel run //deploy:publish            # publish release Flux bundles (infra+migration+app)
bazel run //deploy:publish_preview    # publish preview bundles (disposable Postgres)
```

## Learn more
- [PRD.md](PRD.md) — product definition (what & why).
- [AGENTS.md](AGENTS.md) — contributor/agent guide (layout, conventions, gotchas).
- [docs/](docs/) — [architecture](docs/architecture.md) · [data flows](docs/data-flows.md) ·
  [workflows](docs/workflows.md) · [data model](docs/data-model.md) ·
  [integrations & env vars](docs/integrations.md) · [operations](docs/operations.md) ·
  [testing](docs/testing.md) · [invariants](docs/invariants.md).
