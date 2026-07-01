<!-- Last reviewed against commit 4a51902. -->
# Data model

One Postgres database (`app`). Schema is owned by the API/auth binary and applied with **goose**
migrations in `services/api/migrations/`. All timestamps are `timestamptz`; all UUID PKs default to
`gen_random_uuid()`. Enum-like columns are stored as lowercase **text** and mapped to proto enums in
`internal/api/partner.go`.

## Tables

### `app_users` — authenticated admins (`00001_create_auth_tables.sql`)
| Column | Type | Notes |
|--------|------|-------|
| `id` | uuid PK | default `gen_random_uuid()`; this is the app user id used as the token subject. |
| `external_issuer` | text NOT NULL | OIDC issuer (e.g. `https://accounts.google.com`). |
| `external_subject` | text NOT NULL | OIDC subject (provider user id). |
| `email` | text NOT NULL default `''` | |
| `name` | text NOT NULL default `''` | |
| `created_at`, `updated_at` | timestamptz NOT NULL default `now()` | |

**Identity resolution key:** `UNIQUE (external_issuer, external_subject)` → maps a provider identity
to one app `id`. Enforced by `authdb.UpsertUser` (`internal/auth/db/users.go:8`) via `ON CONFLICT`.

### `auth_sessions` — server-side browser-auth sessions (`00001`)
| Column | Type | Notes |
|--------|------|-------|
| `id` | text PK | session id. |
| `issuer`, `subject` | text NOT NULL | provider identity. |
| `refresh_token` | text NOT NULL | provider refresh token — stays server-side, never sent to the SPA. |
| `claims` | jsonb NOT NULL default `'{}'` | provider claims. |
| `expires_at` | timestamptz NOT NULL | |
| `upstream_expires_at`, `revoked_at` | timestamptz NULL | |
| `created_at`, `updated_at` | timestamptz NOT NULL default `now()` | |

Indexes: `auth_sessions_identity_idx (issuer, subject)`; `auth_sessions_expires_at_idx (expires_at)
WHERE revoked_at IS NULL`. Written by the framework `browserauth.SQLSessionStore`.

### `partners` — the core domain object (`00002_create_partner_tables.sql`)
| Column | Type | Notes |
|--------|------|-------|
| `id` | uuid PK | |
| `name` | text NOT NULL | required. |
| `contact_email` | text NOT NULL | required, validated. |
| `company`, `region`, `notes` | text NOT NULL default `''` | |
| `tier` | text NOT NULL default `'starter'` | enum: `starter` \| `pro` \| `enterprise`. |
| `status` | text NOT NULL default `'pending'` | enum: `pending` \| `active` \| `churned`. |
| `billing_status` | text NOT NULL default `'current'` | enum: `current` \| `past_due` \| `trialing`. |
| `created_at`, `updated_at` | timestamptz NOT NULL default `now()` | |

Indexes: `partners_status_idx (status)`, `partners_created_at_idx (created_at DESC)`.

### `activities` — append-only per-partner log (`00002`)
| Column | Type | Notes |
|--------|------|-------|
| `id` | uuid PK | |
| `partner_id` | uuid NOT NULL | **FK → `partners(id)` ON DELETE CASCADE**. |
| `type` | text NOT NULL default `''` | e.g. `created`, `status_changed`, `submitted`, `note`. |
| `message` | text NOT NULL default `''` | |
| `created_at` | timestamptz NOT NULL default `now()` | |

Index: `activities_partner_idx (partner_id, created_at DESC)`.

## Relationships
```
app_users (id) ──┐  (no FK; token subject only)
                 └▶ used as the caller identity on authenticated RPCs
partners (id) 1 ──── * activities (partner_id)   [ON DELETE CASCADE]
```
There is intentionally **no** FK between `partners` and `app_users` — partners are org-owned, not
per-user.

## DB access layer
Plain `database/sql` + pgx (`_ "github.com/jackc/pgx/v5/stdlib"`), one function per operation,
parameterized SQL. Proto types never appear here.

| Function | File:line | SQL |
|----------|-----------|-----|
| `UpsertUser` | `internal/auth/db/users.go:8` | `INSERT … app_users … ON CONFLICT (external_issuer, external_subject) DO UPDATE … RETURNING id` |
| `Open` / `Ping` | `internal/api/db/db.go:24,31` | open pgx pool; 2s ping. |
| `ListPartners` | `internal/api/db/partner.go:46` | `SELECT … FROM partners ORDER BY created_at DESC` |
| `CountByStatus` | `internal/api/db/partner.go:65` | `SELECT status, count(*) … GROUP BY status` |
| `GetPartner` | `internal/api/db/partner.go:85` | `SELECT … WHERE id=$1` (`sql.ErrNoRows` → NotFound) |
| `GetActivities` | `internal/api/db/partner.go:91` | `SELECT … FROM activities WHERE partner_id=$1 ORDER BY created_at DESC` |
| `CreatePartner` | `internal/api/db/partner.go:116` | `INSERT … coalesce(nullif($n,''),'starter'/'pending'/'current') … RETURNING …` |
| `UpdatePartnerStatus` | `internal/api/db/partner.go:126` | `UPDATE partners SET status=$2, updated_at=now() WHERE id=$1 RETURNING …` |
| `InsertActivity` | `internal/api/db/partner.go:136` | `INSERT INTO activities (partner_id,type,message) RETURNING …` |

## Migrations & ownership
- goose format (`-- +goose Up/Down`), idempotent (`if not exists`); the migrate image globs `*.sql`.
- The `app` database and `app` role are created by the **setup Job** (superuser secret); the API and
  migration Job connect as the non-superuser `app` role. See [operations.md](operations.md) and
  [invariants.md](invariants.md) INV-7.
