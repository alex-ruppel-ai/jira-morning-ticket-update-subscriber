# Jira Morning Ticket Update Subscriber

A Slack bot that delivers a daily AI-generated digest of your Jira tickets every morning. It fetches the full ticket hierarchy under tracked parent issues, collects recent comments and changelogs, and sends a threaded Slack summary powered by OpenAI.

## What It Does

Each morning (at a configurable time), the bot:

1. **Fetches Jira data** — for each tracked parent ticket, it recursively fetches all child issues (subtasks, next-gen children, linked issues) up to 10 levels deep, in parallel. For tickets updated in the last 24h it also pulls recent comments and status/assignee changelog entries.

2. **Builds an excerpt** — the raw Jira data is serialized into a structured text excerpt: parent summary, child statuses, assignees, stale flags (3+ days without update), recent comments, and changelog diffs.

3. **Summarizes with OpenAI** — the excerpt is sent to OpenAI in two separate prompts:
   - **TL;DR** (GPT-4o mini) — a bullet-point summary of _only_ what changed in the last 24h
   - **Work-stream breakdown** (GPT-4o) — per parent ticket, children are grouped into named work streams with a short narrative on current state and what needs to happen next

4. **Posts a threaded Slack message** — the bot posts a title message to the configured channel, then replies in-thread with the TL;DR and a per-parent breakdown. If OpenAI is unavailable, it falls back to template-based messages.

The bot also deduplicates: it records each successful post in MySQL and skips re-posting if it has already run for today.

## Architecture

This app runs on **Applied Intuition's Apps Platform** — a Go + React stack deployed to Google Cloud Run.

```
Jira REST API
      |
      v
jira_update.go         -- recursive ticket/comment/changelog fetch (parallel goroutines)
      |
      v
summarize.go           -- builds text excerpt, calls OpenAI GPT-4o / GPT-4o-mini
      |
      v
slacklib (bot)         -- posts threaded message to Slack channel
      |
      v
db.go (MySQL)          -- records daily post, stores config and tracked tickets
```

### Backend (`main.go`, `api.go`)

- **Gin** HTTP server serves both the Slack webhook endpoints and the React frontend
- **slacklib** handles Slack event routing, message sending, and bot token management
- A background goroutine polls every minute and triggers `runDailyUpdate` once per day at the configured time
- MySQL (Cloud SQL) stores: tracked parent ticket keys, post schedule config (time, timezone, Slack channel), and a daily post log to prevent duplicates

### Frontend (`frontend/`)

- React + TypeScript + Tailwind dashboard for configuring the bot
- Add/remove tracked Jira parent tickets
- Set the Slack channel and daily post time
- Manually trigger a digest run
- View recent event log

### Key Files

| File | Purpose |
|------|---------|
| `jira_update.go` | Jira API calls, recursive child fetching, Slack message builders |
| `summarize.go` | OpenAI prompts and response handling |
| `db.go` | MySQL schema, migrations, and query helpers |
| `api.go` | REST API routes for the React frontend |
| `main.go` | Server startup, Slack handlers, background scheduler |
| `dataapi.go` | Proxies Jira requests through the Apps Platform Data API |

## Apps Platform Integration

### Configuration (`project.toml`)

```toml
name = "jira-morning-ticket-update-subscriber"
enable_secrets = true   # Google Cloud Secret Manager
enable_slack = true     # Slack webhook routing via slack-proxy
enable_mysql = true     # Cloud SQL (MySQL)
```

### Slack Proxy

Apps Platform routes Slack webhooks to the app:

- **Events**: `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/events`
- **Commands**: `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/commands`
- **Interactions**: `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/interactions`

### Secrets

| Secret Name | Description |
|-------------|-------------|
| `<app-name>-slack-bot-token` | Slack Bot OAuth token (`xoxb-...`) |
| `shared-anaheim-service-account-key` | Anaheim user directory service account |
| `OPENAI_API_KEY` | OpenAI API key (set as env var in Cloud Run) |

### Database

Three MySQL tables created automatically on startup:

- `tracked_tickets` — Jira parent keys to monitor (e.g. `ENG-123`)
- `update_config` — post time, timezone, Slack channel, and stored Jira request token
- `daily_update_log` — one row per date; prevents duplicate posts

### Deployment

```bash
apps-platform app deploy
```

## Jira Data Flow

```
tracked_tickets (DB)
        |
        | for each parent key
        v
fetchParentIssue()          GET /rest/api/3/issue/{key}?fields=summary,status,subtasks,issuelinks
        |
        | for each child (JQL: parent={key})
        v
fetchDescendantsRecursive() parallel goroutines, depth-limited to 10
  - fetchChildrenByJQL()    GET /rest/api/3/search/jql?jql=parent%3D{key}
  - fetchComments()         GET /rest/api/3/issue/{key}/comment   (if updated <24h)
  - fetchChangelog()        GET /rest/api/3/issue/{key}/changelog  (if updated <24h)
        |
        v
buildRecentActivityText()   only tickets with comments or changelog entries
buildSingleParentText()     full hierarchy with stale flags
        |
        v
OpenAI GPT-4o-mini          TL;DR: what changed in the last 24h (max 5 bullets)
OpenAI GPT-4o               Work-stream breakdown per parent ticket
        |
        v
bot.SendMessage()           Thread title to Slack channel
bot.SendMessageInThread()   Reply 1: TL;DR
bot.SendMessageInThread()   Reply 2+: per-parent breakdown
        |
        v
dbRecordDailyPost()         Write today's date + Slack thread TS to MySQL
```
