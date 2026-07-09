<!-- Last reviewed against commit 4a51902. -->
# Workflows & state

Runtime control-flow and state machines. For the hop-by-hop data path see
[data-flows.md](data-flows.md); for topology see [architecture.md](architecture.md).

## Partner status lifecycle
`status` is text in `partners`; transitions happen only via `CreatePartner`, `SubmitOnboarding`, and
`UpdatePartnerStatus`. There is no restriction on transitions — any status can move to any other via
`UpdatePartnerStatus`.

```
                (admin CreatePartner / BulkImport)
                          ─▶ active ◀─┐
 (public SubmitOnboarding) ─▶ pending │  UpdatePartnerStatus (any → any)
                                 └─────┴─▶ churned
```
- **Entry as `active`:** `CreatePartner` and `BulkImportPartners` set `STATUS_ACTIVE`
  (`internal/api/partner.go:96,150`).
- **Entry as `pending`:** `SubmitOnboarding` sets `STATUS_PENDING` (`partner.go:172-176`).
- **Any → any:** `UpdatePartnerStatus` (`partner.go:109`) rejects only `STATUS_UNSPECIFIED`
  (`CodeInvalidArgument`) and missing ids (`NotFound`), then writes the new status and appends a
  `status_changed` activity. Every create/change appends to `activities` — see
  [invariants.md](invariants.md) INV-5.

## Subscription lifecycle
```
(none) ──CreateCheckoutSession──▶ [Stripe Checkout] ──checkout.session.completed──▶ active
active ──customer.subscription.updated(past_due|unpaid)──▶ past_due
active/past_due ──customer.subscription.deleted──▶ canceled
```
Stored as `subscription_status` on `partners`: `''` (never subscribed) | `active` | `past_due` |
`canceled`, with `subscription_plan` ∈ `''` | `monthly` | `annual`. Empty maps to the proto
`*_UNSPECIFIED` enum value, which the SPA renders as "Not subscribed".

Only Stripe webhooks advance this state — never the RPC, which just mints a Checkout URL. Stripe
statuses we don't surface (`trialing`, `incomplete`, …) are acknowledged and ignored. Transitions are
idempotent and each state change appends a `subscription` activity (INV-4d, INV-5). Billing is
disabled entirely when the Stripe env is unset; the RPC returns `CodeUnavailable` and the webhook 503s.

## Browser auth session lifecycle
Owned by the framework `browserauth` handler (`internal/auth/browser.go`).
```
anonymous ──/auth/login──▶ Google ──/auth/callback──▶ session created (auth_sessions)
   ▲                                                        │
   │ /auth/logout (LogoutRedirect "/")                      │ /auth/token → 10-min app JWT (in-memory)
   └──────────────── token expiry / logout ◀────────────────┘ /auth/refresh (uses stored refresh token)
```
- App JWTs are short-lived (10-minute TTL, `browser.go:88-93`) and never persisted client-side; the
  SPA `AuthTokenManager` re-fetches from `/auth/token` on demand and dedupes concurrent calls.
- Provider **refresh tokens** live only in `auth_sessions`; the SPA never sees them.
- Invalid/stale OAuth state routes to the framework invalid-state handler (restart login), not a raw
  error page.

## Frontend routing & auth guard
`web/src/App.tsx` renders a `react-router-dom` route table; `useSession` (`web/src/session.ts`)
drives a 4-state machine.
```
useSession: loading ─▶ anonymous ─▶ authenticated
                    └▶ error
```
- **Public routes:** `/onboard` (always) and `/login`.
- **Protected routes** (`/dashboard`, `/partners/new`, `/partners/:id`, `/import`) render inside
  `ProtectedLayout`:
  - `loading` → spinner panel;
  - `anonymous` → `<Navigate to="/login">`;
  - `error` → `Login` with retry;
  - `authenticated` → `AppShell` + `<Outlet>`.
- `/` and unknown paths redirect to `/dashboard`; if already authenticated, `/login` redirects to
  `/dashboard`.

## Bulk import (per-row, best-effort)
`BulkImportPartners` (`internal/api/partner.go:135`) iterates rows; each row is independent:
```
for each row: validate ─▶ ok?  ─yes─▶ CreatePartner(active) ─▶ imported++
                              └─no──▶ append {row, message} to errors (skip)
```
Response `{imported, errors[]}`; a bad row never aborts the batch. Client parses/validates first in
`web/src/lib/csv.ts`; the server re-validates. See [invariants.md](invariants.md) INV-6.

## API startup sequence
`api.Run` (`internal/api/app.go:25`): `httpapp.Init` → `apidb.Open` (logs a warning and continues if
the DB is down) → `appauth.New` (fails startup if auth/OIDC config is missing/invalid) → build bearer
authenticator → register Sample/Partner (protected) + Onboarding (public) + the auth
`ConnectServices` and `/auth` routes → `runtime.NewService(...).Run`. Readiness does **not** wait on
Postgres.
