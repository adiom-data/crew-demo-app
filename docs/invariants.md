<!-- Last reviewed against commit 4a51902. -->
# Invariants

Statements that MUST hold for the system to operate correctly — a fusion of product intent
([PRD.md](../PRD.md)) and technical design. Each notes where it is enforced, or `<UNENFORCED>` /
`<PARTIAL>` when nothing (or only part) of the code guarantees it.

## INV-1: App-token signing key is stable across replicas
- **Statement:** All API replicas MUST sign/verify app tokens with the same `AUTH_PRIVATE_KEY_BASE64`
  (Ed25519 seed).
- **Rationale:** A token minted by one pod must verify on any pod; regenerating the seed invalidates
  outstanding tokens.
- **Source:** PRD §8 (security); [operations.md](operations.md).
- **Enforcement:** `cmd/api/main.go` requires the var (startup fails if absent); stability itself is
  operational — provided by the shared `crew-demo-auth` secret. `<UNENFORCED>` in code.

## INV-2: OAuth state key is stable across replicas
- **Statement:** All API replicas MUST use the same `AUTH_STATE_KEY_BASE64` for browser OAuth state.
- **Rationale:** `/auth/login` and `/auth/callback` can hit different pods; mismatched keys fail with
  `browserauth: invalid state`.
- **Source:** PRD §8; [operations.md](operations.md).
- **Enforcement:** required var (startup fails if absent); `browserauth.CookieStateKeysFromSeedBase64`
  (`internal/auth/browser.go:30`). Cross-replica stability is `<UNENFORCED>` (secret-provided).

## INV-3: Every authenticated user is an admin
- **Statement:** Any successful OIDC login MUST receive scope `sample:user`, and all partner
  management RPCs MUST require exactly that scope — there is no finer role model.
- **Rationale:** Deliberate simplification for the demo; also means there is no per-tenant isolation.
- **Source:** PRD §2, §10.
- **Enforcement:** `dbAuthorizer.Authorize` grants `["sample:user"]` (`internal/auth/authorization.go:37`);
  authenticator requires it (`internal/api/app.go:54`).

## INV-4: Public vs protected is consistent across API and gateway
- **Statement:** An RPC path is reachable unauthenticated **iff** it is registered without
  `tokenissuer.ConnectAuth` in the API **and** marked `"public": true` in `gateway.json`.
- **Rationale:** A mismatch either exposes a protected RPC or breaks a public one. `OnboardingService`
  is a separate service precisely because the gateway marks whole path prefixes.
- **Source:** PRD §7; [architecture.md](architecture.md).
- **Enforcement:** registration `internal/api/app.go:65-85`; gateway `services/gateway/gateway.json`.
  `<PARTIAL>` — the two must be kept in sync by hand; no automated check.

## INV-5: Partner creation and status changes are logged
- **Statement:** Creating a partner, submitting onboarding, and changing status MUST append an
  `activities` row.
- **Rationale:** The activity log is the audit trail shown on partner detail.
- **Source:** PRD §7; [data-flows.md](data-flows.md).
- **Enforcement:** `CreatePartner`→`InsertActivity("created")`, `UpdatePartnerStatus`→`InsertActivity
  ("status_changed")`, `SubmitOnboarding`→`InsertActivity("submitted")` (`internal/api/partner.go`).
  `<PARTIAL>` — **`BulkImportPartners` does not log an activity per imported row** (known gap).

## INV-6: Bulk import is best-effort per row
- **Statement:** An invalid row in `BulkImportPartners` MUST be reported as `{row, message}` and
  skipped, never aborting the batch; valid rows are still imported.
- **Rationale:** Partial CSVs shouldn't fail wholesale.
- **Source:** PRD §7.
- **Enforcement:** per-row loop in `internal/api/partner.go:135` (`BulkImportPartners`).

## INV-7: The app DB role is least-privilege
- **Statement:** The API and migration Jobs MUST connect as the non-superuser `app` role; the `app`
  database MUST be created by the setup Job, not by the app.
- **Rationale:** Blast-radius containment; the app role has no `CREATEDB`/superuser.
- **Source:** PRD §8; [operations.md](operations.md).
- **Enforcement:** `deploy/migrations/setup-job.yaml` (superuser creates DB+role),
  `deploy/infra/postgres-cluster.yaml`; app/migration use `crew-demo-postgres-app`. `<UNENFORCED>` in
  application code (manifest-level).

## INV-8: API readiness does not depend on Postgres
- **Statement:** The API MUST become/stay ready even when Postgres is unavailable; DB errors are
  handled per request.
- **Rationale:** Transient DB outages shouldn't roll pods out of service.
- **Source:** PRD §8; [workflows.md](workflows.md).
- **Enforcement:** no DB readiness check; `apidb.Open` failure only warns (`internal/api/app.go:37-40`);
  handlers return `CodeUnavailable` when `db == nil` (`internal/api/partner.go` `errDatabaseUnavailable`).

## INV-9: Provider refresh tokens never reach the browser
- **Statement:** Provider refresh tokens MUST stay server-side; the SPA holds only short-lived app
  tokens, in memory.
- **Rationale:** Limits token exfiltration from the browser.
- **Source:** PRD §8; [data-flows.md](data-flows.md).
- **Enforcement:** framework `browserauth.SQLSessionStore` persists refresh tokens to `auth_sessions`
  (`internal/auth/browser.go:66`); the SPA `AuthTokenManager` fetches app tokens from `/auth/token`
  only (`web/src/api/clients.ts`).

## INV-10: Gateway backend/issuer names match the deployed API Service
- **Statement:** `gateway.json` `backends[].name`/`url` and `auth.issuer` MUST reference the API
  Kubernetes Service name (`crew-demo-api`).
- **Rationale:** A mismatch breaks routing and token validation. (This was violated after the
  `sample-app`→`crew-demo-app` rename and fixed at commit 4a51902's successor.)
- **Source:** [architecture.md](architecture.md); [integrations.md](integrations.md).
- **Enforcement:** `services/gateway/gateway.json` values vs `deploy/app/base/api-service.yaml`.
  `<UNENFORCED>` — no automated cross-check.

## INV-11: The API re-verifies forwarded bearer tokens
- **Statement:** The API MUST independently verify app tokens against the auth issuer's JWKS, not
  trust the gateway's forwarding.
- **Rationale:** Services reachable in-cluster must not treat unsigned payloads as trusted.
- **Source:** PRD §8.
- **Enforcement:** `tokenissuer.ConnectAuth` + `NewLazyRemoteVerifier(Issuer: AUTH_ISSUER)`
  (`internal/api/app.go:50-53`).
