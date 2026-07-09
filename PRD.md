# On-board (crew-demo-app) — Product Definition Document

**Version:** 1.1
**Status:** Active
**Last reviewed against commit:** 4a51902

## Changelog
| Date | Version | Change | Commit |
|------|---------|--------|--------|
| 2026-07-09 | 1.1 | Adds Stripe subscription billing (monthly/annual checkout + webhook-driven state). | — |
| 2026-06-30 | 1.0 | First full PRD; supersedes the original build brief. Documents the implemented "On-board" portal. | 4a51902 |
| 2026-06-30 | 0.1 | Initial build brief ("Adiom Crew — Demo App Build Brief"). | 373d50e |

## 1. Overview
**On-board** is a small but production-shaped B2B SaaS: a **partner / client onboarding &
management portal**. A company's ops/CS team signs in, onboards partners, manages their lifecycle,
and watches a dashboard; partners can self-submit through a public onboarding form. It is
deliberately vertical-neutral — it stands in for the "onboarding + dashboards + integrations"
problem common to marketplaces, healthtech, fintech, agencies, and edtech.

Its real purpose is to be a **2-minute demo canvas for Adiom Crew** built on the canonical
`adiom-data/framework` stack (Bazel, Go + Connect RPC, a Vite/React SPA, Postgres, BFF/OIDC browser
auth, FluxCD/Kubernetes delivery). It is a fork of `adiom-data/sample-app`, renamed to
`crew-demo-app`, with the sample's single "session" screen grown into the full portal.

## 2. Target audience & personas
- **Admin (primary)** — a company ops / customer-success operator. Signs in with Google, manages
  partners, reviews self-serve submissions, and monitors account health. In this build **every
  authenticated user is an admin** (any successful Google login gets the `sample:user` scope); there
  is no finer-grained role model. See [docs/invariants.md](docs/invariants.md) INV-3.
- **Partner (secondary, light)** — a prospective partner who fills out the **public** onboarding
  form. No login; their submission lands as `pending` for an admin to review.
- **Demo operator / engineer** — the person driving the Adiom Crew demo or extending the app; a heavy
  consumer of this documentation.

## 3. User goals
- **Admin:** see partner counts and health at a glance; add a partner quickly; drill into a partner's
  profile and activity; change a partner's status; import many partners at once from a CSV; triage
  self-serve submissions.
- **Partner:** register interest without an account and get a clear "received / pending" confirmation.

## 4. Key features
All implemented as of commit 4a51902 unless labelled otherwise.
- **Real admin login** via Google OIDC (framework BFF browser auth; app tokens kept in memory,
  refresh tokens server-side).
- **Dashboard** — summary cards (Total / Active / Pending) plus a partners table (name, company,
  region, tier, status).
- **Add Partner** — validated form that writes a partner (created as `active`).
- **Partner detail** — profile, billing status, activity log, and status-change actions (Pending /
  Active / Churned), each logged as an activity.
- **Bulk import** — client-side CSV parse (`name,contact_email,company,region,tier`) → server
  validates each row and returns a per-row error report; valid rows are imported.
- **Public onboarding form** — unauthenticated self-submit that creates a partner with status
  `pending`.
- **Subscription billing (Stripe)** — from partner detail an admin starts a real Stripe Checkout
  session on a **monthly** or **annual** plan. Stripe webhooks drive the partner's subscription state
  (`active` / `past_due` / `canceled`) and each change is logged as an activity. Billing is optional:
  with no Stripe credentials configured the rest of the app runs unchanged.
- **Seed data** — an idempotent seeder (`cmd/seed`) inserts ~30 realistic partners so the dashboard
  looks alive.

## 5. Core entities
Grounded in the schema; see [docs/data-model.md](docs/data-model.md).
- **Partner** — id, name, contact_email, company, region, tier (starter/pro/enterprise), status
  (pending/active/churned), billing_status (current/past_due/trialing), notes, created_at, plus Stripe
  subscription fields (stripe_customer_id, stripe_subscription_id, subscription_plan, subscription_status).
- **Activity** — id, partner_id, type, message, created_at. Append-only log per partner.
- **App user** (`app_users`) — an authenticated admin, keyed by OIDC `(issuer, subject)`.
- **Auth session** (`auth_sessions`) — server-side browser-auth session holding the provider refresh
  token and claims.

## 6. Core workflows
See [docs/workflows.md](docs/workflows.md) and [docs/data-flows.md](docs/data-flows.md).
- **Admin login → token → authenticated call** (OIDC BFF flow).
- **Public onboarding submission** → partner created as `pending`.
- **Partner management** — create, view detail, change status (with activity logging), bulk import.
- **Dashboard load** — session check + partner list with summary counts.

## 7. Functional requirements
- The system MUST authenticate admins via an external OIDC provider and MUST gate all partner
  management RPCs behind a verified bearer token carrying the `sample:user` scope.
- The public onboarding RPC MUST be callable **without** authentication and MUST create partners with
  status `pending`.
- Partner and CSV input MUST be validated (non-empty name; syntactically valid contact email);
  invalid bulk-import rows MUST be reported per-row without aborting the whole import.
- Every partner status change and creation MUST append an activity record.
- Subscription state MUST be changed only by verified Stripe webhooks, never by the client: the
  billing RPC only mints a Checkout URL. The webhook MUST verify its HMAC signature over the raw
  request body and MUST be idempotent under Stripe's at-least-once redelivery.
- The dashboard MUST show Total / Active / Pending counts consistent with the partners table.

## 8. Non-functional requirements
- **Security:** app tokens are short-lived (10-minute TTL) and signed with a stable Ed25519 seed;
  browser refresh tokens live only server-side in `auth_sessions`; the gateway validates tokens and
  forwards the verified bearer to the API, which independently re-verifies via issuer JWKS.
- **Availability / graceful degradation:** API readiness MUST NOT depend on transient Postgres
  availability; DB errors are handled per-request. This demo runs **1 replica** each for the API and
  gateway; the framework convention for production is ≥2 replicas with `maxUnavailable: 0` /
  `maxSurge: 1` for zero-downtime rolling updates.
- **Portability:** Base Kubernetes manifests omit `namespace` and `HTTPRoute.hostnames` so the same
  bundles deploy into any tenant namespace. The **prod** overlay (`deploy/app/overlays/prod`) pins
  the release host `t-crew-demo.infrapad.ai`; the **preview** overlay stays host-less so ephemeral
  tenant namespaces bind their own host.
- **Observability:** OpenTelemetry traces/metrics to the namespace collector; structured JSON logs to
  stdout. Secrets and raw dependency errors MUST NOT appear in logs or client responses.
- **Scale/performance:** `<MISSING>` — no explicit targets defined (this is a demo app).
- **Compliance:** `<MISSING>` — none stated.

## 9. Success metrics
`<MISSING>` — none defined in-repo. The implicit goal is a smooth 2-minute Adiom Crew demo; there are
no instrumented product KPIs.

## 10. Risks & dependencies
- **External dependency:** a reachable OIDC provider (Google) with a registered web client; the API
  fails startup if OIDC/auth config is missing or the provider is unreachable.
- **External dependency:** Postgres (CloudNativePG in release, a disposable Deployment in preview).
- **External dependency:** provider-managed platform pieces — `otel-collector` service, Gateway API
  ingress, and pre-created Kubernetes secrets (`crew-demo-auth`, `crew-demo-postgres-*`).
- **Delivery dependency:** Bazel + `adiom-data/bazel-rules` (Flux OCI bundles) and the pinned
  `ghcr.io/adiom-data/components/{gateway,goosemigrate}` base images.
- **Assumption:** every authenticated user is trusted as an admin (no role separation) — acceptable
  for a demo, a risk for real multi-tenant use. See [docs/invariants.md](docs/invariants.md) INV-3.
- **Key-management risk:** regenerating `AUTH_PRIVATE_KEY_BASE64` invalidates outstanding app tokens;
  regenerating `AUTH_STATE_KEY_BASE64` breaks in-flight logins. Both must stay stable across replicas.
