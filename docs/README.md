<!-- Last reviewed against commit 4a51902. Authoritative review marker for all docs. -->
# Documentation index

Topic-per-file: each fact lives in one place; other docs link to it. Start at
[../AGENTS.md](../AGENTS.md) (agent guide) or [../PRD.md](../PRD.md) (product).

| Doc | Read this when … |
|-----|------------------|
| [architecture.md](architecture.md) | you need the component map / topology / API surface. |
| [data-flows.md](data-flows.md) | you're tracing how a request or entity moves end-to-end. |
| [workflows.md](workflows.md) | you need runtime control-flow / state machines (status lifecycle, auth session, routing). |
| [data-model.md](data-model.md) | you need tables, columns, relationships, or the DB access layer. |
| [integrations.md](integrations.md) | you need an env var, a secret, or a third-party service. |
| [operations.md](operations.md) | you're building, running locally, migrating, or deploying. |
| [testing.md](testing.md) | you're running tests or adding a test target. |
| [invariants.md](invariants.md) | you need the rules that must always hold (and the known gaps). |

The consolidated conventions that used to live in `docs/patterns.md` now sit in
[../AGENTS.md](../AGENTS.md) (code-organization principles), [operations.md](operations.md)
(deploy/DB ownership), and [invariants.md](invariants.md) (the MUST-hold rules).
