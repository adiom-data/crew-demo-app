# sample-app

A Bazel-only sample app with a Go API/auth server, Postgres-backed auth state,
a Vite/React SPA, `ghcr.io/adiom-data/components/gateway` and
`ghcr.io/adiom-data/components/goosemigrate` component images, and Flux OCI
bundle publishing through `adiom-data/bazel-rules`.

Architecture patterns and deployment assumptions are documented in
[docs/patterns.md](docs/patterns.md). Read that before changing bundle
boundaries, database setup, migration Jobs, or gateway health behavior.

Repository layout:

- `cmd/` contains small runnable binary entrypoints.
- `internal/api/` contains the sample API service and binary composition.
- `internal/api/db/` contains API-owned database access helpers.
- `internal/auth/` contains browser auth, token exchange, token issuing, and
  user authorization.
- `internal/auth/db/` contains auth-owned database access helpers.
- `services/api/migrations/` contains service-owned app/auth database Goose
  migrations.
- `services/gateway/` contains the gateway image wrapper and config.
- `web/` contains the Vite/React SPA.

Useful targets:

```sh
bazel build //cmd/api:image
bazel build //cmd/migrate:image
bazel build //services/gateway:image
bazel build //deploy:infra_deploy
bazel build //deploy:migration_deploy
bazel build //deploy:app_deploy
bazel run //deploy:publish_all
bazel run //deploy:publish_preview_all
```

The gateway base image is pinned from `ghcr.io/adiom-data/components/gateway:v0.0.1`
to `sha256:2e04c398ee2463d2090ab9ca18d004ee68dcb9336be33ca0ec67aae13494691b`.
The migration image layers service SQL onto
`ghcr.io/adiom-data/components/goosemigrate:v0.0.1`, pinned to
`sha256:4728e49ee1c35474aab45218cc8b589667a7853d7a94475c97883268ae2aaf46`.

For browser OIDC auth, configure `AUTH_ISSUER`, `OIDC_ISSUER`,
`OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `AUTH_PRIVATE_KEY_BASE64`,
and `AUTH_STATE_KEY_BASE64`. The API fails startup when required auth
configuration is missing or invalid.
For native/mobile credential exchange, expose `adiom.auth.v1.AuthService` and
set `OIDC_ALLOWED_AUDIENCES` to include any additional client IDs that may
present provider ID tokens.

Kubernetes secrets are intentionally not checked into `deploy/`.
`sample-app-auth` must be provided by the deployment environment in the target
namespace. The app also relies on CloudNativePG-generated database secrets,
described below.

Bootstrap `sample-app-auth` with these keys:

- `OIDC_ISSUER`: OIDC provider issuer URL. For Google, use
  `https://accounts.google.com`.
- `OIDC_CLIENT_ID`: OAuth/OIDC web client ID from the provider. For Google,
  this is the client ID ending in `.apps.googleusercontent.com`.
- `OIDC_CLIENT_SECRET`: OAuth/OIDC web client secret from the provider.
- `AUTH_PRIVATE_KEY_BASE64`: base64-encoded 32 random bytes. This is the stable
  Ed25519 seed used to sign app tokens.
- `AUTH_STATE_KEY_BASE64`: base64-encoded 32 random bytes. This is the stable
  browser OAuth state seed used across API replicas.
- `OIDC_ALLOWED_AUDIENCES`: optional comma-separated additional OIDC client IDs
  allowed to exchange provider ID tokens, usually native/mobile client IDs.

For the Google OAuth web client, configure the authorized redirect URI as
`https://<app-hostname>/auth/callback`, matching the public hostname that
serves this app.

Generate the app-owned random secrets as stable values:

```sh
# AUTH_PRIVATE_KEY_BASE64, Ed25519 seed: 32 random bytes, standard base64.
openssl rand -base64 32

# AUTH_STATE_KEY_BASE64, browser OAuth state seed: 32 random bytes, standard base64.
openssl rand -base64 32
```

Both generated values should normally be 44 characters with trailing `=`
padding. Keep them stable across restarts and deploys; regenerating
`AUTH_PRIVATE_KEY_BASE64` invalidates outstanding app tokens, and regenerating
`AUTH_STATE_KEY_BASE64` invalidates in-flight browser login callbacks.

The deployment platform should create a Kubernetes Secret equivalent to:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sample-app-auth
type: Opaque
stringData:
  OIDC_ISSUER: https://accounts.google.com
  OIDC_CLIENT_ID: <web-oauth-client-id>
  OIDC_CLIENT_SECRET: <web-oauth-client-secret>
  AUTH_PRIVATE_KEY_BASE64: <base64-32-random-bytes>
  AUTH_STATE_KEY_BASE64: <base64-32-random-bytes>
  # Optional; omit when only the web client exchanges provider ID tokens.
  OIDC_ALLOWED_AUDIENCES: <comma-separated-client-ids>
```

`AUTH_ISSUER` is configured in the deployment manifest for this in-cluster
sample; set it to the issuer URL reachable by token verifiers.
CloudNativePG bootstraps a dummy `bootstrap` database owned by `app` and
generates the `sample-postgres-app` secret. The `app` role is not a superuser
and does not get `CREATEDB`. The infra setup job uses the stock Postgres image
and `psql` with CNPG's generated `sample-postgres-superuser` secret to create
the real `app` database owned by `app`.

Protobuf and Connect stubs are generated with Buf:

```sh
buf generate
```

The gateway validates app tokens from the API auth issuer and forwards the
verified bearer token to the BFF API unchanged (`auth_forwarding:
"verified_bearer"`), so it does not need its own internal-JWT signing key. The
API also verifies forwarded app tokens through the auth issuer's metadata and
JWKS; verifier discovery is lazy so a new pod does not need to call its own
Service before it starts serving.

Deployment is split into three Flux OCI bundles:

- `//deploy:infra_deploy` owns the CloudNativePG cluster.
- `//deploy:migration_deploy` runs lightweight database setup jobs and app
  database Goose migrations. It is marked `force = True`.
- `//deploy:app_deploy` owns the gateway and API workloads.

This repository is intended to be a canonical sample rather than an in-place
database upgrade recipe. If you previously deployed the older shape where CNPG
bootstrapped the `app` database directly, recreate the sample Postgres cluster
and its PVCs before applying the new bundle sequence.

The API and gateway Kubernetes probes use framework gRPC health services. The
API does not gate readiness on transient database availability; database errors
are handled at request time so the service can recover when Postgres returns.

The checked-in Kubernetes resources intentionally omit `metadata.namespace` and
`HTTPRoute.hostnames`, so the bundle can be applied to any namespace and bound
to hostnames by the environment. If `PUBLIC_BASE_URL` is not set, browser auth
uses the framework request-scoped redirect resolver to derive its OAuth
redirect base URL from forwarded request host/proto headers; auth requests fail
if the gateway does not provide a usable host.

Observability is provider-owned. Framework telemetry emits traces and metrics
to the namespace collector default `http://otel-collector:4318`. The sample
deployment sets `OTEL_SERVICE_NAME=sample-api`; application logs remain
structured stdout/stderr and are collected by the provider's Kubernetes log
pipeline.
