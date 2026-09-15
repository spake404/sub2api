# Sub178Agents

Sub178Agents is a public source snapshot of Sub2API 0.1.178 with optional
OpenAI OAuth HTTP Responses subagent session modes. It is based on
[Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) and retains the upstream
license and notices.

Repository: https://github.com/AR307/sub178agents

## Architecture

- `backend/cmd/server`: Go application entry point and dependency wiring.
- `backend/internal/handler` and `backend/internal/server`: HTTP APIs, authentication, and routing.
- `backend/internal/service`: account scheduling, upstream adapters, sessions, and usage accounting.
- `backend/internal/repository`, `backend/ent`, and `backend/migrations`: PostgreSQL persistence and Redis integration.
- `frontend/src`: Vue 3, TypeScript, and Vite administration and user interfaces.
- `deploy` and `Dockerfile`: example configuration and source-based container builds.

## Subagent Session Modes

Configure `extra.codex_fingerprint_mode` on an OpenAI OAuth account. Both modes
are available in account creation, editing, and bulk editing. They are opt-in.

| Mode | Behavior |
| --- | --- |
| `subagent` | Stable account parent session and independent child threads; preserves the caller's prompt cache key. |
| `subagent_v2` | Native-layout metadata, account parent session as the prompt cache key, and conditional parent/root turn references. |

Both modes apply only to HTTP `POST /v1/responses`. WebSocket, compact,
Messages, Chat Completions, and API-key account paths are unchanged.

An account seed determines its stable logical parent. An inbound thread and
the gateway API key determine a stable child thread within that account.
Follow-up calls reuse the child; another client thread receives another child.
The window ID is the child thread ID followed by `:0`.

Requests without thread or session identifiers get a request-local child;
internal retries reuse it. Turn State is isolated by mode, account, and child.
No parent agent is started, no parent history is loaded, and no additional
spawn API is called. Upstream quota or cache treatment is not guaranteed by
these metadata fields.

## Development

See [DEV_GUIDE.md](DEV_GUIDE.md) for local prerequisites, source builds, and
focused HTTP forwarding tests. The upstream installation links and images in
the original README describe upstream Sub2API, not a built image of this fork.
Build this repository from source to include the subagent modes.

## Public Source Policy

This repository starts with a clean source-snapshot commit. Internal operational
history, deployment inventories, live account data, raw HAR captures, server
backups, and developer credentials are not part of the public history.

Configuration examples and synthetic test fixtures are retained. The original
working repository and deployment data are separate and are not changed by
publishing this snapshot. Never commit `.env`, runtime configuration, OAuth
tokens, API keys, database dumps, or request captures containing user data.

## Change Log

### 2026-09-15

- Published the 0.1.178 source snapshot with selectable subagent v1 and v2 modes.
- Retained stable session mapping, per-child Turn State, and HTTP forwarding tests.
- Retained the outer client-metadata string-type fix.
- Replaced internal operating notes with public development documentation.
- Expanded exclusions for local credentials, captures, backups, and deployment artifacts.
- Preserved upstream source licensing; did not copy private commit history.
- Re-ran the focused subagent, fingerprint, and Turn State Go tests on the public source snapshot; they passed.
- Removed embedded Google OAuth client IDs and secrets from this public edition; operators supply their own environment configuration. OpenAI session behavior is unchanged.
- Updated Google OAuth fixtures and verified authorization configuration, token HTTP exchange, and refresh scenarios with synthetic credentials.

### Validation Scope

The release includes HTTP forwarding tests for child creation and continuation,
turn mapping, account/API-key isolation, streaming, and passthrough behavior.
Historical full-suite failures in
`TestContentModerationRuntimeSnapshotRefreshFailureKeepsStaleConfig` and
`TestGrokQuotaServiceQueryQuotaFreeFallsBackToGrok45` were outside the subagent
change; do not interpret focused test success as a full-suite pass.
