<!-- Last reviewed against commit 4a51902. -->
# Operations runbook

Bazel is the only build tool. Delivery is FluxCD reconciling OCI bundles built by
`adiom-data/bazel-rules`. Env vars/secrets live in [integrations.md](integrations.md).

## Build
```sh
bazel build //...                       # everything: Go, web bundle, OCI images
bazel build //cmd/api:image             # API OCI image (distroless base)
bazel build //cmd/migrate:image         # goose migration image (SQL layered on goosemigrate)
bazel build //services/gateway:image    # gateway image (SPA + gateway.json on the gateway base)
bazel build //web:build                 # Vite bundle → web/dist
bazel run  //cmd/api:load               # load an image into local Docker (also :migrate, :gateway)
```
- **Codegen:** `buf generate` writes `gen/go/**` and `web/src/gen/**`. BUILD files under `gen/go/**`
  are hand-maintained — after adding a proto file, add its `.pb.go`/`.connect.go` to `srcs` by hand.
- **Gazelle:** `bazel run //:gazelle` for plain-Go BUILD files. The root `BUILD.bazel` sets
  `# gazelle:proto disable_global`; do not remove it or gazelle will generate conflicting proto rules
  that drop the `//gen/go/sample/v1:samplev1` deps.
- **OCI base images** need registry auth for `gcr.io` (`gcloud auth login`) on a cold cache.

## Local development
Full stack locally with real Google OIDC. Ports: Postgres `55432`, API `8080`, Vite `5173`.

1. **Postgres**
   ```sh
   docker run -d --name crew-demo-pg -e POSTGRES_PASSWORD=pass -e POSTGRES_DB=app \
     -p 55432:5432 postgres:18
   ```
2. **Migrate + seed**
   ```sh
   goose -dir services/api/migrations postgres \
     "postgres://postgres:pass@localhost:55432/app?sslmode=disable" up
   PGHOST=localhost PGPORT=55432 PGDATABASE=app PGUSER=postgres PGPASSWORD=pass \
     bazel run //cmd/seed
   ```
3. **Google OAuth client** — a "Web application" client with redirect URI
   `http://localhost:5173/auth/callback`. Generate two stable seeds: `openssl rand -base64 32` each.
4. **API** (`:8080`)
   ```sh
   PGHOST=localhost PGPORT=55432 PGDATABASE=app PGUSER=postgres PGPASSWORD=pass PGSSLMODE=disable \
   PORT=8080 AUTH_ISSUER=http://localhost:8080/auth AUTH_INSECURE_COOKIES=true \
   PUBLIC_BASE_URL=http://localhost:5173 OIDC_ISSUER=https://accounts.google.com \
   AUTH_PRIVATE_KEY_BASE64=<seed1> AUTH_STATE_KEY_BASE64=<seed2> \
   OIDC_CLIENT_ID=<id> OIDC_CLIENT_SECRET=<secret> \
   go run ./cmd/api
   ```
   Note `PORT=8080` (not `:8080` — the framework prepends the colon).
5. **SPA** (`:5173`)
   ```sh
   cd web && pnpm install && pnpm dev
   ```
   `web/vite.config.mjs` proxies `/auth`, `/sample.v1.*`, and `/adiom.auth.v1.*` to
   `API_PROXY_TARGET` (default `http://localhost:8080`). Open http://localhost:5173.

The public `/onboard` form works without login; the dashboard requires Google sign-in. To tear down:
`pkill -f cmd/api; pkill -f vite; docker rm -f crew-demo-pg`.

### Stripe billing (optional locally)
Omit the Stripe env and the API still runs — billing just reports `Unavailable`. To exercise it:

1. `stripe listen --forward-to localhost:8080/stripe/webhook` — copy the printed `whsec_…`.
2. Restart the API with `STRIPE_SECRET_KEY=sk_test_…`, `STRIPE_WEBHOOK_SECRET=whsec_…` (from step 1),
   `STRIPE_PRICE_MONTHLY=price_…`, `STRIPE_PRICE_ANNUAL=price_…`.
3. Open a partner → **Subscribe monthly** → pay with `4242 4242 4242 4242`, any future expiry/CVC/ZIP.
4. The partner flips to `Monthly · Active` with a `subscription` activity row.
   `stripe events resend <id>` must not add a second row (INV-4d).

`PUBLIC_BASE_URL=http://localhost:5173` (already in the block above) is what the Checkout
success/cancel URLs are built from.

## Migrations
- goose SQL in `services/api/migrations/`, named `NNNNN_description.sql`, `-- +goose Up/Down`,
  idempotent (`if not exists`). The migrate image globs `*.sql` onto `/app/migrations`.
- In-cluster, the **migration Job** runs goose as the `app` role against database `app`; goose tracks
  applied versions in `goose_db_version`, so reruns are safe.
- Locally, apply with the `goose` CLI (above) or run the migrate image against your DB.

## Deploy (FluxCD OCI bundles)
Built and published from `deploy/BUILD.bazel`. Bundles, applied in order:

| Bundle | Target | Contents | Notes |
|--------|--------|----------|-------|
| infra | `//deploy:infra_deploy` | CloudNativePG `Cluster` (`crew-demo-postgres`). | long-lived; never force-apply. |
| preview-infra | `//deploy:preview_infra_deploy` | disposable `postgres:18` Deployment (emptyDir). | preview only; swaps for `infra`. |
| migration | `//deploy:migration_deploy` | setup Job + migration Job. | `force = True` (Job pod templates are immutable), `stamp = True`. |
| seed | `//deploy:seed_deploy` | demo-data seed Job (`crew-demo-seed-app`) running the `crew-demo-app-seed` image. | **preview only** — in `publish_preview`, not the release set; `force = True`, `stamp = True`. |
| app (release) | `//deploy:app_deploy` | API + gateway Deployments/Services + `HTTPRoute`, rendered through `deploy/app/overlays/prod` (pins host `t-crew-demo.infrapad.ai`). | `stamp = True`; images `crew-demo-app-{api,gateway}`. |
| app (preview) | `//deploy:app_preview_deploy` | Same base via `deploy/app/overlays/preview` (no host patch — stays portable). | `stamp = True`; used by `publish_preview`. |

Publish:
```sh
bazel run //deploy:publish            # release: infra + migration + app  (manifest_tag "release")
bazel run //deploy:publish_preview    # preview: preview-infra + migration + seed + app
```
**Order at reconcile:** setup Job (creates `app` DB + role as superuser) → migration Job (goose as
`app`) → seed Job (preview only) → app workloads. The seed Job has no wait loop of its own; it exits
non-zero until Postgres is up *and* the `partners` table exists, relying on `restartPolicy: OnFailure`
+ `backoffLimit: 20` to retry until it succeeds. The seeder is idempotent (skips if any partners
already exist), so retries and re-reconciles are safe. Base manifests intentionally omit
`metadata.namespace` and
`HTTPRoute.hostnames` so the same bundles bind to any tenant namespace/host; the `overlays/prod`
overlay pins the release host `t-crew-demo.infrapad.ai`, while `overlays/preview` leaves it unset.

Image reference stamping uses `tools/status.sh` (emits `STABLE_GIT_COMMIT`,
`STABLE_REFERENCE_PREFIX` default `ghcr.io/adiom-data`).

## Database ownership (release)
- CloudNativePG bootstraps a dummy `bootstrap` DB owned by `app`, which yields the generated
  `crew-demo-postgres-app` secret without making the real `app` DB a CNPG bootstrap artifact.
- The **setup Job** uses the generated `crew-demo-postgres-superuser` secret to create the real `app`
  database owned by the `app` role. The `app` role is **not** a superuser and has no `CREATEDB`.
- API and migration Jobs connect only as `app`. See [invariants.md](invariants.md) INV-7.

## Health & observability
- API and gateway expose framework gRPC health services for Kubernetes probes. API readiness does
  **not** depend on transient Postgres availability.
- Telemetry (OTLP) and structured stdout logs are framework-owned; the platform collects them. Do not
  add observability backends to these bundles or log secrets/raw dependency errors.
