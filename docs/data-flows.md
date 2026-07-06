<!-- Last reviewed against commit 4a51902. Bump when revised against newer code. -->
# Data flows

End-to-end paths, entry → exit. Topology is in [architecture.md](architecture.md); state/control-flow
is in [workflows.md](workflows.md); this file names the exact hop at each step.

## Flow 1 — Admin login → app token → authenticated call
The identifier that threads the flow: OIDC `(issuer, subject)` → `app_users.id` (UUID) → token
`subject` → `samplev1.User.Id`.

```
Browser ─▶ /auth/login ─▶ Google ─▶ /auth/callback ─▶ dbAuthorizer.Authorize
   │                                                        │ upsert app_users, grant sample:user
   │                                                        ▼
   │                                               auth_sessions (refresh token, server-side)
   ▼
/auth/token ─(app JWT)─▶ SPA ─(Bearer)─▶ Gateway ─(verified bearer)─▶ API interceptor ─▶ handler
```
1. **`/auth/login`** — framework `browserauth` handler (wired in `internal/auth/browser.go:42-74`)
   redirects to Google with PKCE + `access_type=offline` + `prompt=consent`
   (`browser.go:51-54`). Redirect URI = `PUBLIC_BASE_URL`/`PROXY_REDIRECT_URL` + `/auth/callback`,
   else derived from request headers.
2. **`/auth/callback`** — framework exchanges the code, then calls the app authorizer
   `dbAuthorizer.Authorize` (`internal/auth/authorization.go:17`): it reads `external.Issuer/Subject`
   and upserts the user via `authdb.UpsertUser` (`internal/auth/db/users.go:8`), returning
   `Identity{Subject: app_users.id, Scopes: ["sample:user"], Attributes: {email,name}}`
   (`authorization.go:35-42`). The provider **refresh token** is persisted to `auth_sessions` by the
   framework `SQLSessionStore` (`browser.go:66`).
3. **`/auth/token`** — the framework mints a short-lived (10-min) app JWT signed by the Ed25519 seed
   `AUTH_PRIVATE_KEY_BASE64` (issuer `tokenIssuer`, `internal/auth/browser.go:88-93`). The SPA's
   `AuthTokenManager` (`web/src/api/clients.ts`) holds it in memory only.
4. **Authenticated RPC** — SPA sends `Authorization: Bearer <jwt>`. The gateway validates it against
   the API auth issuer and forwards the verified bearer. The API interceptor
   `tokenissuer.ConnectAuth(authenticator)` (registered `internal/api/app.go:70,77`) verifies via the
   **lazy remote verifier** against `AUTH_ISSUER` (`app.go:50-53`), requires scope `sample:user`
   (`app.go:54`), and maps claims → `*samplev1.User{Id: claims.Subject,…}` stored on context
   (`app.go:55-62`).
5. **Handler** reads identity with `tokenissuer.AuthValueFromContext[*samplev1.User]` from the request
   context (e.g. `internal/api/sample.go:18`).

Sync boundary: everything here is synchronous request/response (no queues).

## Flow 2 — Public onboarding submission
```
Browser (/onboard, no token) ─▶ OnboardingService.SubmitOnboarding ─▶ CreatePartner (status=pending)
                                                                    └▶ InsertActivity("submitted")
                                                     ▶ returns { partnerId }
```
1. **SPA** `web/src/routes/PublicOnboarding.tsx` calls the **unauthenticated** onboarding client
   (`web/src/api/partnerClient.ts` `createOnboardingClient`, no auth interceptor). Path
   `/sample.v1.OnboardingService/SubmitOnboarding` is marked `"public": true` in `gateway.json` and
   registered without the auth interceptor (`internal/api/app.go:79-84`).
2. **`onboardingService.SubmitOnboarding`** (`internal/api/partner.go:162`) validates name + email
   (`validatePartnerInput`), then `apidb.CreatePartner` with `Status = STATUS_PENDING`
   (`partner.go:172-178`) → row lands as `pending`.
3. **`apidb.InsertActivity(…, "submitted", …)`** (`partner.go:182`) appends an activity.
4. Returns `SubmitOnboardingResponse{PartnerId}`. The partner now appears on the admin dashboard as
   Pending, awaiting a status change (Flow 4).

## Flow 3 — Dashboard load
```
useSession ─▶ SampleService.GetSession ─▶ (identity + DB ping)
Dashboard  ─▶ PartnerService.ListPartners ─▶ ListPartners + CountByStatus ─▶ cards + table
```
1. **`useSession`** (`web/src/session.ts`) gets a token, then calls `SampleService.GetSession`
   (`internal/api/sample.go:17`): extracts the user from context, pings the DB, returns
   `{authenticated, user, database}`.
2. **`Dashboard`** (`web/src/routes/Dashboard.tsx`) calls `PartnerService.ListPartners`
   (`internal/api/partner.go:26`): `apidb.ListPartners` (all partners, newest first) +
   `apidb.CountByStatus` → response `{partners, total, active, pending}` (`partner.go:39-44`). The
   `active`/`pending` cards come from the `CountByStatus` map, `total` from the row count.

## Flow 4 — Partner management (create / detail / status / bulk)
- **Create** — `AddPartner.tsx` → `PartnerService.CreatePartner` (`partner.go:82`): validate →
  `CreatePartner` with `Status = STATUS_ACTIVE` → `InsertActivity("created")` → SPA redirects to the
  new partner's detail page.
- **Detail** — `PartnerDetail.tsx` → `GetPartner` (`partner.go:51`): `GetPartner` + `GetActivities`.
- **Status change** — status buttons → `UpdatePartnerStatus` (`partner.go:109`):
  `apidb.UpdatePartnerStatus` (`sql.ErrNoRows` → NotFound) → `InsertActivity("status_changed", …)` →
  SPA reloads the detail.
- **Bulk import** — `BulkImport.tsx` parses the CSV client-side (`web/src/lib/csv.ts`
  `csvToPartnerRows`), maps tier text → enum (`web/src/lib/format.ts` `parseTier`), and sends all
  rows to `BulkImportPartners` (`partner.go:135`). The server validates each row; failures become
  `{row, message}` entries in the response and the row is skipped; valid rows are inserted as
  `active`. The SPA renders `imported` + the per-row error report.

## Flow 5 — AdiomBot agent MCP query
```
AdiomBot worker ─▶ POST /mcp (streamable HTTP, no auth) ─▶ grpcmcp handler
                                                        └▶ (self, reflection) AgentQueryService.ListPartners
                                                     ▶ returns partners + counts as an MCP tool result
```
1. The **AdiomBot worker** (integrations repo), when this repo's stable environment has Agent MCP
   enabled with the app's public URL, connects to `https://<host>/mcp` and lists tools.
2. **`/mcp`** is served by `agentMCPHandler` (`internal/api/agentmcp.go`), which lazily builds a
   grpcmcp server on first request: it self-dials the in-process Connect API via gRPC reflection and
   exposes only `sample.v1.AgentQueryService` as MCP tools.
3. A tool call proxies to **`AgentQueryService.ListPartners`** (`internal/api/agentquery.go`), which
   reads via the same DB helpers as PartnerService and returns partner data. Unauthenticated and
   read-only by design (see [invariants.md](invariants.md) INV-4b).

## Seeding (out-of-band)
`cmd/seed/main.go` opens the DB directly (env `PG*`), and if `partners` is empty inserts ~30 partners
+ activities via the same `apidb` helpers (idempotent: skips when rows exist). Not part of any RPC
path; run via `bazel run //cmd/seed` locally. In **preview** environments it also runs automatically:
the `seed` bundle ships a `crew-demo-seed-app` Job (image `crew-demo-app-seed`) that runs after
migrations — see [operations.md](operations.md#deploy-fluxcd-oci-bundles). Release/prod is never
auto-seeded.
