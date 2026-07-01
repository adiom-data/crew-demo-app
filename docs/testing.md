<!-- Last reviewed against commit 4a51902. -->
# Testing

Two suites: Go unit tests (via Bazel `go_test`) and a Vitest frontend suite (via a Bazel
`vitest_test` target). Tests are unit-level and hit no database or network.

## Run everything
```sh
bazel test //...
```
This runs all four test targets:

| Target | File(s) | Covers |
|--------|---------|--------|
| `//cmd/api:api_test` | `cmd/api/main_test.go` | env → config parsing (e.g. `PROXY_REDIRECT_URL` trimming). |
| `//internal/api:api_test` | `internal/api/partner_test.go` | `validatePartnerInput`, tier/status/billing enum↔text round-trips, proto mapping. |
| `//internal/api/db:db_test` | `internal/api/db/db_test.go` | `postgresURL` DSN escaping. |
| `//web:test` | `web/src/**/*.test.ts[x]` | CSV parser (`csv.test.ts`), formatters (`format.test.ts`), `AddPartner` render + validation. |

## Run a single test
```sh
# One Go target, one test function:
bazel test //internal/api:api_test --test_filter=TestValidatePartnerInput
go test ./internal/api/ -run TestValidatePartnerInput   # without Bazel

# The Vitest suite (all 21 tests) via Bazel:
bazel test //web:test

# Frontend directly (faster iteration), a single file or name:
cd web && pnpm test                       # vitest run
cd web && pnpm exec vitest run src/lib/csv.test.ts
cd web && pnpm exec vitest run -t "parses simple rows"
cd web && pnpm exec tsc --noEmit          # typecheck only
```

## Conventions
- **Go:** in-package `*_test.go`, table-driven, standard `testing`; no DB/testcontainers — test pure
  functions (validation, mapping, DSN building). Add a `go_test` `srcs` entry (and any `deps`) in the
  package `BUILD.bazel`.
- **Frontend (Vitest):** config in `web/vitest.config.ts` (jsdom, `globals`, `include:
  src/**/*.test.{ts,tsx}`). `web/vitest.setup.ts` registers `@testing-library/jest-dom` matchers by
  extending `expect` explicitly — required because the repo uses an isolated pnpm node-linker, so
  importing `@testing-library/jest-dom/vitest` cannot resolve `vitest` from the hoisted store.
- **Vitest under Bazel:** `//web:test` (`web/BUILD.bazel`) lists source + test files via `glob` and
  the needed `:node_modules/*` links (including `vitest` itself). New third-party test deps must be
  added there.
- No JS coverage gate; frontend correctness is Vitest + `tsc --noEmit` + the production `vite build`.
