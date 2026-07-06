<!-- Last reviewed against commit 4a51902. Bump when revised against newer code. -->
# Architecture

## System topology
Three deployable workloads plus Postgres. In production the **gateway** is the front door; locally
the Vite dev server proxies to the API and there is no gateway.

```
Browser ─▶ Gateway ─(verified bearer)─▶ API (+ auth, one binary) ─▶ Postgres
                │                              │
                │ serves SPA (/app/web/dist)   └─ OIDC ─▶ Google (accounts.google.com)
                ▼
        Gateway API ingress (HTTPRoute; prod overlay pins t-crew-demo.infrapad.ai)
```

- **Gateway** (`services/gateway/`) — the pinned `ghcr.io/adiom-data/components/gateway` image
  configured by `services/gateway/gateway.json`. It serves the SPA from `/app/web/dist`, validates
  app tokens against the API auth issuer, marks some routes public, and forwards the **verified
  bearer** to the API (`auth_forwarding: "verified_bearer"`). It holds no signing key.
- **API + auth** (`cmd/api`, `internal/api`, `internal/auth`) — a single Go binary that both serves
  the Connect RPC services and hosts the BFF browser-auth endpoints under `/auth`. Assembled with
  `framework/httpapp` (`internal/api/app.go:25` `Run`).
- **Migrate** (`cmd/migrate`) — the goose image (SQL layered on `components/goosemigrate`); runs as a
  Kubernetes Job, not a long-lived service.
- **Postgres** — CloudNativePG cluster in release, a disposable Deployment in preview. See
  [data-model.md](data-model.md) and [operations.md](operations.md).

## Components (API process)
| Component | File(s) | Responsibility |
|-----------|---------|----------------|
| Entrypoint | `cmd/api/main.go` | Parse env → `api.Config`; call `api.Run`. |
| Composition | `internal/api/app.go` | `httpapp.Init`, open DB, build authenticator, register services + auth routes, run. |
| SampleService impl | `internal/api/sample.go` | `GetSession` — returns the caller's identity + DB health. |
| PartnerService impl | `internal/api/partner.go` | `ListPartners`, `GetPartner`, `CreatePartner`, `UpdatePartnerStatus`, `BulkImportPartners`. |
| OnboardingService impl | `internal/api/partner.go` | `SubmitOnboarding` (public). Enum↔text mapping + validation helpers live here too. |
| AgentQueryService impl | `internal/api/agentquery.go` | `ListPartners` — read-only surface behind `/mcp` (public, unauthenticated). Delegates to the same DB helpers as PartnerService. |
| Agent MCP endpoint | `internal/api/agentmcp.go` | Lazily builds the `/mcp` streamable-HTTP handler via [grpcmcp](https://github.com/adiom-data/grpcmcp), discovering `AgentQueryService` through in-process gRPC reflection. |
| API DB helpers | `internal/api/db/{db.go,partner.go}` | `Open`/`Ping` + partner/activity SQL. Proto-free. |
| Auth service | `internal/auth/service.go` | Wires the token issuer, browser-auth handler, and the credential-exchange Connect service. |
| Browser auth | `internal/auth/browser.go` | `framework/browserauth` handler for `/auth/*`; Google OAuth options; `SQLSessionStore`. |
| Authorizer | `internal/auth/authorization.go` | `dbAuthorizer.Authorize` — upsert user, grant scope `sample:user`. |
| Auth DB helpers | `internal/auth/db/users.go` | `UpsertUser` (identity resolution). |

## API surface (proto package `sample.v1`)
Defined in `proto/sample/v1/{sample.proto,partner.proto,agentquery.proto}`; Go stubs in
`gen/go/sample/v1`. Two services are **authenticated** (require a bearer token with scope
`sample:user`); the rest are **public**.

| RPC | Service | Auth | Purpose |
|-----|---------|------|---------|
| `GetSession` | `sample.v1.SampleService` | bearer | Return caller identity + DB status. |
| `ListPartners` | `sample.v1.PartnerService` | bearer | Partners + Total/Active/Pending counts. |
| `GetPartner` | `sample.v1.PartnerService` | bearer | One partner + its activity log. |
| `CreatePartner` | `sample.v1.PartnerService` | bearer | Create a partner (status `active`). |
| `UpdatePartnerStatus` | `sample.v1.PartnerService` | bearer | Change status; log an activity. |
| `BulkImportPartners` | `sample.v1.PartnerService` | bearer | Import rows; per-row error report. |
| `SubmitOnboarding` | `sample.v1.OnboardingService` | **public** | Self-serve; creates status `pending`. |
| `ListPartners` | `sample.v1.AgentQueryService` | **public** | Read-only partner data for the AdiomBot agent (also reachable via `/mcp`). |
| `POST /mcp` | streamable-HTTP MCP | **public** | grpcmcp endpoint exposing `AgentQueryService` tools to the AdiomBot worker. |
| `ExchangeCredential` | `adiom.auth.v1.AuthService` | **public** | Native/mobile provider-token → app token (framework). |
| `/auth/{login,callback,token,logout,refresh}` | browser BFF | **public** | OIDC browser flow (framework). |

Registration (protected = registered *with* `tokenissuer.ConnectAuth`): `internal/api/app.go:65-85`.
The same public/protected split is mirrored per path prefix in `services/gateway/gateway.json`. These
two must agree — see [invariants.md](invariants.md) INV-4.

## Frontend (web/)
Vite + React 19 SPA, ConnectRPC client, `@adiom-data/framework-web` for auth. Client-side routing via
`react-router-dom`. Key modules:
- `web/src/App.tsx` — route table + `ProtectedLayout` (auth guard → `AppShell` + `<Outlet>`).
- `web/src/session.ts` — `useSession` hook (token → `GetSession`).
- `web/src/api/{clients.ts,sampleClient.ts,partnerClient.ts}` — one shared `AuthTokenManager`; the
  onboarding client is unauthenticated.
- `web/src/routes/*` — `Dashboard`, `AddPartner`, `PartnerDetail`, `BulkImport`, `PublicOnboarding`,
  `Login`.
- `web/src/lib/{csv.ts,format.ts}` — CSV parsing + enum/date formatting.
Control-flow detail is in [workflows.md](workflows.md); the exact hops are in [data-flows.md](data-flows.md).

## Cross-cutting
- **Telemetry & health** are framework-owned (`httpapp`): OpenTelemetry OTLP export and gRPC health
  services for Kubernetes probes. Apps add no local logging/trace middleware. See
  [integrations.md](integrations.md).
- **Graceful degradation:** API readiness does not depend on Postgres; handlers tolerate a missing DB.
- **Codegen:** buf produces `gen/go/**` and `web/src/gen/**`; BUILD files under `gen/go/**` are
  hand-maintained (`# gazelle:proto disable_global`). See [operations.md](operations.md).
