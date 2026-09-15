# Public Development Guide

## Repository And Prerequisites

- Repository: https://github.com/AR307/sub178agents
- Upstream: https://github.com/Wei-Shaw/sub2api
- Base version: 0.1.178
- Backend: Go 1.26.6, Gin, Ent, PostgreSQL, and Redis.
- Frontend: Node.js 20 or newer, pnpm 9, Vue 3, TypeScript, and Vite.

Use your own local PostgreSQL and Redis instances. Generate private credentials;
do not reuse example passwords or connect development tools to a live database.
Copy and customize `deploy/.env.example` locally when using the Compose templates.
Runtime configuration and credentials must remain untracked.

## Google OAuth Configuration

The public edition does not embed Google OAuth client credentials. To enable
Antigravity or the shared Gemini CLI OAuth flow, provide authorized credentials
to the application process before startup:

```text
ANTIGRAVITY_OAUTH_CLIENT_ID
ANTIGRAVITY_OAUTH_CLIENT_SECRET
GEMINI_CLI_OAUTH_CLIENT_ID
GEMINI_CLI_OAUTH_CLIENT_SECRET
```

For Docker Compose, pass these names through the application's `environment`
section. A value in the local Compose `.env` file alone is not automatically
exported into the container. Gemini's existing custom OAuth configuration is
also supported. Missing shared Gemini client credentials produce a configuration
error. OpenAI account setup and subagent v1/v2 behavior do not require these
Google credentials.

## Build This Fork

The upstream installer and prebuilt upstream images do not include this fork's
subagent changes. Build this source instead:

```bash
git clone https://github.com/AR307/sub178agents.git
cd sub178agents
docker build -t sub178agents:local .
```

For a local source build without Docker:

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run build
cd ../backend
go build -tags embed -o sub2api ./cmd/server
```

The frontend build writes the embedded UI to `backend/internal/web/dist`.
The server's initial setup wizard is available on port 8080 when no runtime
configuration exists. Supply the database, Redis, and admin settings for your
own environment. See `deploy/config.example.yaml` for configuration details.

For development, run `go run ./cmd/server` from `backend` and `pnpm dev`
from `frontend`.

## Subagent Implementation

- `backend/internal/service/openai_subagent_session.go`: input identity capture and v1 mapping.
- `backend/internal/service/openai_subagent_session_v2.go`: v2 metadata and output headers.
- `backend/internal/service/openai_codex_fingerprint.go`: account mode and fingerprint integration.
- `backend/internal/service/openai_codex_turn_state.go`: per-mode/account/child state isolation.
- `backend/internal/service/openai_gateway_forward.go` and `openai_gateway_passthrough.go`: HTTP forwarding integration.
- `frontend/src/components/account`: create, edit, and bulk-edit account mode selection.
- `backend/internal/service/testdata/codex_subagent_v2.json`: synthetic forwarding fixture, not a raw client capture.

V1 and v2 are separate opt-in account modes. Do not change the mode of unrelated
accounts while testing. Neither mode loads a parent's context or creates an
actual upstream parent agent.

## Verification

Run the focused HTTP forwarding scenarios from `backend`:

```bash
go test -tags=unit ./internal/service -run 'Test.*(Subagent|CodexFingerprint|CodexTurnState)' -count=1
```

These tests use HTTP requests and mock receivers to inspect forwarded headers,
body metadata, independent child threads, follow-up calls, and streaming paths.
A mock upstream test does not replace testing against an authorized real account.

Run frontend checks from `frontend`:

```bash
pnpm run typecheck
pnpm exec vitest run src/components/account/__tests__/CreateAccountModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts
pnpm run build
```

For broader backend regression coverage:

```bash
go test -tags=unit ./...
go test -tags=integration ./...
```

Integration tests require their local service/container dependencies. Known
historical full-suite failures are recorded in `Summary.md`; investigate failures
without modifying unrelated behavior just to make a focused release pass.

## Contributions And Publication

Create a feature branch, make focused changes, update `Summary.md`, and commit.
Keep the existing module path and upstream notices unless intentionally changing
the module layout. Keep `pnpm-lock.yaml` synchronized with dependency changes.

Do not publish runtime `config.yaml`, `.env`, API keys, OAuth access/refresh tokens,
private certificates, databases, request logs, HAR files, or server backups.
Use synthetic fixtures for tests and scan staged files before a public push.

Build images locally before deploying to resource-constrained servers. Back up
your database and existing runtime configuration before replacing an application.
Never replace a working database with example data during an application update.
