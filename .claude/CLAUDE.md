# Agentic App Builder Template

This template is used to build and deploy web apps on Applied's Apps Platform (Go + React, Cloud Run).

## Required Workflow for New Features or Apps

When a user asks you to build something **new** (a new feature, new page, new app), you MUST follow this workflow in order. Do not skip steps. Use the `Skill` tool to invoke each skill by name.

For bug fixes, refactors, or changes to existing code, this workflow does not apply — use your judgment.

1. **Invoke `v1-scoping` (phase 1)** — Assess feasibility and set a rough v1 boundary BEFORE brainstorming
2. **Invoke `brainstorming`** — Design the full solution within the confirmed scope. Pass the v1-scoping output (feasibility answers + rough v1 boundary) as context so brainstorming does not re-ask those questions.
   > **OVERRIDE:** The `brainstorming` skill's default terminal state invokes `writing-plans` directly. In this template, do NOT follow that default. After brainstorming completes its design doc, return to this workflow and continue with step 3.
3. **Invoke `v1-scoping` (phase 2)** — Cut the brainstormed design to the smallest deployable v1
4. **Invoke `user-journey`** — Walk through exactly how a user interacts with the v1
5. **Invoke `writing-plans`** — Produce a sequenced, task-level implementation plan

## Why This Order Matters

- Phase 1 of v1-scoping catches infeasible requests before wasting time designing them
- Brainstorming runs on a confirmed, feasible scope — not wishful thinking
- Phase 2 of v1-scoping prunes the full design down to what can actually be shipped and tested
- User journey validates the pruned design is coherent from a real user's perspective
- writing-plans converts the validated design into actionable tasks

## User Context

Users of this template are **nontechnical** and cannot run commands. All changes must be made directly to the code. Do not instruct users to run any CLI commands.

**Bias toward UI-level suggestions.** Users are nontechnical — they think in terms of buttons, pages, forms, and visible behavior, not backend logic or data structures. When brainstorming or proposing ideas:
- Lead with what the user will *see and interact with* (new tabs, dashboards, input forms, status indicators, modals)
- Describe Slack interactions in terms of user-facing commands or messages, not handler wiring
- Avoid leading with backend/API concerns — surface those only after establishing what the UI experience looks like
- If a feature has both UI and backend work, anchor the description on the UI outcome first

## Verifying Your Work

After making any changes to Go files, you MUST run `go build ./...` to confirm the code compiles before considering the task done. Do not claim work is complete until this passes with no errors.

## Making Changes

After making changes, make sure to run apps-platform app deploy --no-build to run and deploy your app

### Testing via Deployment

You cannot test Slack webhooks or end-to-end behavior locally. To verify your implementation works correctly:

1. **Invoke the `apps-platform` skill** — follow its deploy flow to build and push the app
2. **Spawn a subagent to tail logs** — after deploy completes, spawn a dedicated subagent whose sole job is to run `apps-platform app logs` and watch for evidence that the feature behaves correctly (API calls succeeded, handlers fired, data was read/written)
3. **Verify correctness from logs** — do not claim the feature works until the log-tailing subagent returns confirmation that the expected log lines appeared

### Logging Requirements

Add **verbose structured logging** throughout the codebase so the log-tailing subagent can reason about what happened. Specifically:

- **Every button press / Slack action:** Log the action ID, user ID, and any payload values when a `bot.Action(...)` or `bot.ViewSubmission(...)` handler fires
- **Every API call (outbound):** Log the method, URL/endpoint, request parameters, and response status code before and after each external API call (Nango, Google APIs, Slack API, MySQL queries)
- **Every API handler (inbound):** Log the route, method, and key query/body params at the top of each Gin handler in `api.go`
- **Decision points:** Log which branch was taken whenever the code makes a meaningful conditional decision (e.g., "user has token", "cache hit", "fallback triggered")
- **Errors:** Always log the full error with context before returning it

Use `log.Printf("[ComponentName] message: %v", value)` or the `cloudlogger` package consistently. Prefix log lines with a bracketed component name (e.g., `[SlackAction]`, `[GoogleSheets]`, `[API]`) so the subagent can grep for relevant lines quickly.

## Go Dependencies

Do **not** modify `go.mod` in any way — do not add, remove, or tidy it. Do not run `go mod tidy`, `go mod download`, or `go get`. All necessary packages are already vendored — use only what is already present in the `vendor/` directory.

Before importing any package, verify it exists under `vendor/`. Prefer standard library packages over third-party ones. For example:
- **Unique IDs**: use `crypto/rand` (stdlib) — do NOT import `github.com/google/uuid`
- **JSON**: use `encoding/json` (stdlib)
- **Time**: use `time` (stdlib)

## Architecture

This is a full-stack Slack bot template: **Go backend + React frontend**, deployed to Google Cloud Run via Apps Platform.

You only support Google Sheets, Google Docs, Google Drive, Google Calendar, Gmail, Slack (user OAuth), Jira, Confluence, and Anaheim. If someone wants a different integration — push back and offer alternatives. Use the `integration-capabilities` skill to determine what is and isn't possible within each supported integration.

YOU MUST NOT INSTALL EXTERNAL DEPENDENCIES.

### Request Flow

**Development:**
```
Browser (port 3000) → Vite proxy → Go backend (port 8081) → Slack API
```

**Production:**
```
Slack → slack-proxy service → Cloud Run (Go binary with embedded React build)
```

### Backend (`main.go`, `api.go`)

- **Gin** HTTP framework handles all routes
- `main.go` wires Slack event handlers using `slacklib` (the vendored Apps Platform Slack library)
- `api.go` contains REST API handlers for the React frontend
- In production, Go serves the embedded React build (`frontend/dist/`) via `embed.FS`
- In dev (`DEV=true`), Go proxies to Vite instead

**Slack event registration pattern:**
```go
bot.OnMention(func(ctx *slacklib.MentionContext) { ... })
bot.OnDM(func(ctx *slacklib.DMContext) { ... })
bot.Command("/name", func(ctx *slacklib.CommandContext) { ... })
bot.ViewSubmission("callback_id", func(ctx *slacklib.ViewContext) { ... })
bot.Action("action_id", func(ctx *slacklib.ActionContext) { ... })
```

**Slack webhook endpoints** (routed through `slack-proxy` in production):
- `POST /slack/events`, `POST /slack/commands`, `POST /slack/interactions`

### Frontend (`frontend/src/`)

- React 18 + TypeScript + Tailwind CSS, built with Vite
- `App.tsx`: root component with tab-based navigation state and a `StatusContext` for bot connectivity
- `Sidebar.tsx`: nav with 6 tabs and bot connection status indicator
- Components poll `/api/events` every 5 seconds for real-time updates (no WebSocket)
- `frontend/src/lib/api.ts`: shared API utilities, including `isBuildingResponse()` to gracefully handle 503s during Go startup

### Event Log (`eventlog/`)

Thread-safe in-memory ring buffer (max 50 events). Used by all Slack handlers to record activity visible in the Dashboard tab.

### Key Dependencies

- `go.apps.applied.dev/lib` — provides `slacklib`, `anaheim` (user directory), and `cloudlogger`
- `github.com/gin-gonic/gin` — HTTP server
- `github.com/slack-go/slack` — Slack API client (used internally by slacklib)

### Integration Documentation

Before using any third-party or internal API (e.g. Anaheim, Nango, Google Sheets), read the relevant documentation file in `.claude/docs/blueprints/static/`. Do not guess field names or method signatures — look them up in the docs first.

For Data API integrations (Sheets, Docs, Drive, Calendar, Gmail, Slack, Jira, Confluence):
1. Invoke the `integration-capabilities` skill — confirms what is possible with default scopes and provides pushback language for unsupported requests
2. Read `.claude/docs/integrations.md` — documents the correct short names to use in API URLs and provides request examples

### MySQL Database

MySQL is **enabled** (`enable_mysql = true` in `project.toml`). The database is ready to use after deploying.

**Environment variables (set automatically in Cloud Run):**
- `MYSQL_INSTANCE_CONNECTION_NAME`
- `MYSQL_DB_USER` = `agentic-app-template-sa`
- `MYSQL_DB_NAME` = `agentic_app_template`

**Before writing any MySQL code:**
1. Read `.claude/docs/ai/mysql-example.md` — complete working example with schema, seed data, connection wiring, and API routes
2. Read `.claude/docs/ai/database-patterns.md` — connection patterns for both local and Cloud SQL

**Key rules:**
- Branch on `MYSQL_INSTANCE_CONNECTION_NAME` to handle local vs. cloud connection
- Run `CREATE TABLE IF NOT EXISTS` migrations on startup
- Never modify `go.mod` — verify `github.com/go-sql-driver/mysql` and `cloud.google.com/go/cloudsqlconn` exist under `vendor/` before importing
- **You cannot test MySQL locally.** To verify database changes work, deploy with `apps-platform app deploy` and then read logs with `apps-platform app logs` to confirm the connection and migrations succeeded

### Secrets & Config

- `project.toml` controls Apps Platform features (`enable_slack`, `enable_secrets`, `enable_mysql`, etc.)
- Bot token loaded via: explicit config → Secret Manager (`<app-name>-slack-bot-token`) → `SLACK_BOT_TOKEN` env var

### Platform Deployment

- Apps deploy to Google Cloud Run via `apps-platform app deploy`
- Go backend with embedded React frontend (via `go:embed`)
- See `docs/ai/` for architecture patterns, database connections, secrets, and deployment

### Important Limitation

Slack webhooks do **not** work when running locally — they require a deployed Apps Platform URL for routing.
