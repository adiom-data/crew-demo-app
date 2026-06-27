# Project Patterns

This repository is a canonical sample for a Bazel-built web/API application
deployed to Kubernetes with FluxCD. Prefer clear, repeatable patterns over
one-off deployment shortcuts.

## Deployment Environment

The expected deployment environment is:

- Kubernetes.
- FluxCD reconciling OCI-hosted deployment bundles.
- CloudNativePG managing Postgres clusters and generated database credentials.
- Gateway API routing traffic to the gateway.

Bazel builds application images and Flux OCI bundles. Flux then reconciles the
rendered Kubernetes resources from those bundles into the target namespace. The
checked-in manifests intentionally omit namespace and hostname details so the
same bundles can be used in different tenant environments.

This matters for Jobs: Flux can detect a changed Job manifest, but Kubernetes
does not allow most `Job.spec.template` fields to be updated in place. A bundle
that contains Jobs must either use forced apply semantics or create new Job
names for changed Jobs.

## Bundle Boundaries

- Bazel is the only build tool used for project artifacts.
- Deployable application artifacts are pushed as OCI images and Flux OCI
  bundles through `adiom-data/bazel-rules`.
- Deploy is split into three ordered bundles:
  - `infra`: long-lived infrastructure, currently the CloudNativePG cluster.
  - `preview_infra`: disposable preview infrastructure, currently a plain
    Postgres Deployment backed by `emptyDir` to avoid per-preview PVCs.
  - `migration`: rerunnable Kubernetes Jobs for database setup and schema
    migrations.
  - `app`: runtime workloads, currently the API and gateway.
- Only the `migration` bundle uses `force = True`. This is intentional because
  Kubernetes Job pod templates are immutable and Jobs need to be recreated when
  their scripts or images change.
- Do not put long-lived resources like the Postgres `Cluster` in a forced
  bundle.
- Preview publish targets may swap the long-lived infra bundle for
  preview-specific disposable infrastructure, but should keep the same service
  and secret names consumed by migration and app bundles.
- User-facing Deployments should run at least two replicas with
  `maxUnavailable: 0`, `maxSurge: 1`, and readiness probes so normal rolling
  updates keep an old ready pod serving until a new pod is ready.

## Observability

- Observability collection and backend storage are provider-owned. Application
  bundles should not install SigNoz, VictoriaMetrics, VictoriaLogs, or other
  observability backends.
- Each tenant namespace is expected to have a provider-managed OTel Collector
  service named `otel-collector` accepting OTLP/HTTP on port `4318`, matching
  the framework telemetry default.
- Applications emit traces and metrics via standard OpenTelemetry OTLP to the
  namespace collector. The provider collector or gateway assigns trusted tenant
  identity before forwarding data to shared backends.
- Applications log structured JSON to stdout/stderr. They should not use an
  OTel log exporter. The provider's Kubernetes log pipeline collects,
  enriches, and forwards pod logs.
- Application-controlled resource attributes are useful for service identity,
  version, and environment, but must not be trusted as tenant identity.

## Job Idempotency

Every Job in the forced `migration` bundle must be idempotent.

- Setup Jobs must check existing state before creating or changing resources.
- Goose migration Jobs are idempotent because Goose records applied migrations
  in `goose_db_version`.
- A Job rerun should either make the missing change or exit successfully when
  the desired state already exists.

## Database Ownership

- CloudNativePG bootstraps a dummy `bootstrap` database owned by the app role.
  This gives us a generated `sample-postgres-app` secret without making the app
  database itself a CNPG bootstrap artifact.
- CNPG superuser access is enabled so the operator generates
  `sample-postgres-superuser`.
- Preview infra may replace CNPG with disposable Postgres, but the deployment
  environment must still provide DB credential secrets compatible with the
  shared migration and app bundles.
- Database creation Jobs connect with the generated superuser secret.
- Application workloads and schema migration Jobs connect with the generated
  app secret.
- The app role should not be a superuser and should not have `CREATEDB`.
- Application databases are created by setup Jobs and owned by the app role.

## Migrations

- Service-owned database migrations live with the owning service under
  `services/<service>/migrations`.
- Service-owned database access helpers should live with service
  implementation code, such as `internal/<service>/db`.
- Each database should have its own migration Job in `deploy/migrations`.
- Each migration Job should connect to exactly one database.
- Migration images should layer service SQL migrations onto the shared
  `ghcr.io/adiom-data/components/goosemigrate` component image. The component
  runs migrations packaged at the well-known `/app/migrations` directory and
  receives only database connection details from the environment.
- Migration Jobs should be deployed in an order where required databases and
  roles already exist. The migration runner should fail normally when the
  database is unavailable rather than hiding ordering problems with long
  in-process waits.

## Service Construction

- Go HTTP/Connect services should use `framework/httpapp` for server assembly.
- `cmd/<name>` packages should remain small entrypoints that call service code.
- `cmd/<name>` packages should own process environment parsing and pass typed
  config into internal packages. Internal service packages should not read env
  directly except for narrowly scoped helper binaries that are themselves
  drivers.
- Long-running HTTP services should initialize framework runtime with
  `httpapp.Init` and construct servers with `runtime.NewService` so telemetry
  providers and Connect instrumentation use the same process-level wiring.
- Let `httpapp.Init` own signal handling and use `runtime.Context()` for
  service lifetime and any setup that should stop with the process.
- Construct outbound generated Connect clients with
  `runtime.ConnectHTTPClient()` and `runtime.ConnectClientOptions()` so client
  RPC telemetry and trace propagation use the initialized framework providers.
- Construct raw outbound HTTP clients with `runtime.HTTPClient()` when request
  telemetry and propagation are desired.
- Service implementation should live outside `cmd/`, under the appropriate
  service or internal package.
- Register generated Connect services through `httpapp.ConnectHandler`.
- Rely on framework-provided HTTP, Connect, logging, trace, and metric
  instrumentation instead of adding local middleware for the same signals.
- Register framework readiness checks for startup/configuration conditions that
  should remove a pod from service. Do not use readiness for transient
  dependencies the service can handle and recover from at request time.
- Kubernetes probes should use the framework gRPC health services exposed by
  `httpapp`.

## Gateway

- The gateway validates tokens and forwards verified bearer credentials to the
  BFF/API.
- Gateway configuration lives with the gateway service wrapper under
  `services/gateway`.
- Gateway container images should be built from the pinned
  `ghcr.io/adiom-data/components/gateway` base image.
- The gateway should stay a validation and routing layer. Business logic, auth
  mediation, and BFF behavior belong behind the gateway in application
  services.
- Gateway routes should explicitly mark public paths, such as browser auth
  routes.
- Authenticated backend routes should forward the verified bearer credential to
  the API instead of minting a separate gateway-internal token.
- APIs should still verify forwarded bearer credentials before deriving user
  identity. The gateway is the routing enforcement point, but services should
  not treat unsigned JWT payloads as trusted input if they are reachable inside
  the cluster. Prefer verifier paths that use the auth issuer's metadata and
  JWKS rather than sharing private signing-key state with API handlers.

## Auth

- Browser auth is owned by the auth service behind the gateway. In this sample
  the auth service runs in the same binary as the API service, but it lives in
  its own package because it has a separate service boundary.
- SPA auth flows should use the BFF browser auth endpoints under `/auth`.
  Generic SPA OIDC clients are only appropriate if the app intentionally moves
  to browser-owned OIDC tokens.
- SPA calls to application APIs should use generated Connect clients.
- SPAs should keep app bearer tokens in memory only. A small token manager may
  cache the app token, parse its expiration, deduplicate `/auth/token` calls,
  and clear local state on logout, but provider refresh tokens remain
  server-side in the DB-backed browser auth session.
- Keep reusable SPA auth glue separate from app-specific API clients. Import
  browser auth helpers from `@adiom-data/framework-web/auth` instead of copying
  token-management code into the app.
- The gateway validates app-issued tokens using the API auth issuer metadata.
- Browser OIDC is configured through Kubernetes secrets consumed by the auth
  service.
- Native/mobile clients should use provider-native auth, then call
  `adiom.auth.v1.AuthService/ExchangeCredential` with a provider ID token. The
  service verifies the provider token against the configured allowed audiences
  and returns a short-lived app token.
- Token signing keys must be stable across API pod restarts and replicas in any
  shared environment. `AUTH_PRIVATE_KEY_BASE64` is required and should be
  provided by a Kubernetes secret.
- Browser OAuth state cookies are signed and encrypted. Their codec keys must
  also be stable across API pod restarts and replicas, otherwise login and
  callback requests can hit different pods and fail with `browserauth: invalid
  state`. Configure these with framework
  `browserauth.CookieStateKeysFromSeedBase64` using `AUTH_STATE_KEY_BASE64`;
  do not derive them from process-local random defaults in a replicated
  deployment.
- Browser OAuth callbacks with missing, stale, or already-used state should use
  the framework invalid-state handler to restart login instead of surfacing a
  raw `browserauth: invalid state` error page.
- `AUTH_ISSUER` is required and must be the issuer URL reachable by components
  that verify app tokens, such as the gateway.
- The process should fail startup when required auth configuration is missing
  or invalid. It should not silently disable browser auth or generate ephemeral
  signing keys in shared deploys.
- Browser OAuth should pass `PROXY_REDIRECT_URL` to the framework in preview
  environments so the provider uses a stable proxy callback while the app
  callback remains request-scoped state. Otherwise use `PUBLIC_BASE_URL` when
  configured.
- If neither explicit redirect base is configured, browser auth should use the
  framework request-scoped redirect resolver to derive redirect URLs from
  forwarded host/proto headers. Auth requests should fail rather than use a fake
  fallback URL when those headers are unavailable. Production-like environments
  should prefer an explicit public base URL when one is available.
- Auth/session tables are service-owned schema. In this sample they live with
  the app database migrations because the API and auth services share one
  deployable binary and database.
- Logs and user-facing responses should not include secret values or raw
  dependency errors that may contain connection details. Prefer redacted
  operator logs and generic client-facing error messages.

## Health

- The API and gateway use framework-provided gRPC health services for
  Kubernetes probes.
- API readiness should not depend on transient Postgres availability. Database
  outages should be handled by request paths and recover when the database
  becomes available again.
- Additional health surfaces should be added only for integrations that cannot
  use the framework health services.

## Portability

- Kubernetes resources intentionally omit `metadata.namespace`.
- HTTPRoutes intentionally omit `hostnames`.
- Environment-specific namespace and hostname binding should happen outside
  this sample.
