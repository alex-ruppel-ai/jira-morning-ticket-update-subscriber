# Jira Daily Update Bot — Design

**Date:** 2026-03-23
**Status:** Approved

---

## Overview

A feature that generates and posts automatic daily Jira status updates to a configured Slack channel. Updates are delivered in two forms: a condensed summary as the main Slack message, and a detailed breakdown posted as a thread reply. All configuration (tracked tickets, post time, target channel) is managed through a new Settings tab in the web UI and persisted in MySQL.

---

## User Experience

### Settings Tab (Web UI)

A new "Daily Update" tab in the sidebar with two panels:

**Tracked Tickets panel:**
- Input field: paste a Jira ticket key (e.g. `ADP-123`) and click "Add"
- Each tracked ticket shows its Jira summary (fetched on add) and a remove button
- Stored in MySQL `tracked_tickets` table

**Schedule panel:**
- Post time: time picker (HH:MM)
- Timezone: dropdown (PT / ET / UTC)
- Slack channel: text field (e.g. `#adp-daily`)
- "Save" button to persist settings
- "Post Now" button to trigger the update immediately on demand

---

### Slack Output

**Condensed message (main post):**
```
Daily Update — Mon Mar 23

ADP-123 · Sensor Fusion Infra  [In Progress]
  8 done  3 in progress  1 blocked

ADP-456 · Perception Pipeline  [In Review]
  12 done  2 in progress

Blockers: ADP-201 (blocked), ADP-389 (no update in 3d)
Recent (24h): ADP-201 -> Done, ADP-312 assigned to @sara
```

**Detailed breakdown (thread reply):**
```
Full breakdown — ADP-123

Children:
  ADP-201  Lidar driver update    [Blocked]  @james
  ADP-202  Unit tests             [Done]     @sara
  ...

Recent comments:
  ADP-201: "Waiting on vendor firmware" — james, 2h ago

Changelog (24h):
  ADP-202: Done <- In Progress (sara)
  ADP-312: Assigned to sara
```

---

## Architecture

### MySQL Schema

```sql
CREATE TABLE IF NOT EXISTS tracked_tickets (
    id INT AUTO_INCREMENT PRIMARY KEY,
    jira_key VARCHAR(50) NOT NULL UNIQUE,
    summary VARCHAR(500),
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS update_config (
    id INT AUTO_INCREMENT PRIMARY KEY,
    post_time VARCHAR(5) NOT NULL DEFAULT '09:00',  -- HH:MM
    timezone VARCHAR(50) NOT NULL DEFAULT 'America/Los_Angeles',
    slack_channel VARCHAR(100) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS daily_update_log (
    id INT AUTO_INCREMENT PRIMARY KEY,
    post_date DATE NOT NULL UNIQUE,
    posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    slack_ts VARCHAR(100)  -- timestamp of the Slack message, for threading
);
```

### Backend API Routes (api.go)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/tickets` | List tracked tickets |
| POST | `/api/tickets` | Add a tracked ticket (fetches summary from Jira) |
| DELETE | `/api/tickets/:key` | Remove a tracked ticket |
| GET | `/api/update-config` | Read schedule settings |
| PUT | `/api/update-config` | Save schedule settings |
| POST | `/api/trigger-update` | Manually trigger update (used by "Post Now") |
| POST | `/internal/jobs/check-daily-update` | Called by cron every minute |

### Cron Job

- **Schedule:** `* * * * *` (every minute)
- **Endpoint:** `POST /internal/jobs/check-daily-update`
- **Logic:**
  1. Load `update_config` from MySQL
  2. Parse configured time in configured timezone
  3. If `now` (in that timezone) is within 1 minute of `post_time` AND today's date is not in `daily_update_log` → proceed
  4. Otherwise → return 200 early
  5. Fetch Jira data, build messages, post to Slack
  6. Insert row into `daily_update_log` with the Slack message timestamp

### Jira Data Fetching

For each parent ticket key in `tracked_tickets`:
1. `GET /api/integration/jira/rest/api/3/issue/{key}?fields=summary,status,subtasks` — get parent + child keys
2. For each child key: `GET /api/integration/jira/rest/api/3/issue/{childKey}?fields=summary,status,assignee,updated` — status and assignee
3. For children updated in the last 24h:
   - `GET /api/integration/jira/rest/api/3/issue/{childKey}/comment` — recent comments
   - `GET /api/integration/jira/rest/api/3/issue/{childKey}/changelog` — field change history
4. Identify blockers: issues where `status.name == "Blocked"` or `updated` is older than 72h

**Important Jira API rules (from integration docs):**
- Always use `/rest/api/3/search/jql` (not `/rest/api/3/search` — returns 410)
- JQL must include at least one filter (e.g. `issueKey in (ADP-123, ADP-456)`)

### Message Building

**Condensed message** includes:
- Per-parent: ticket key, summary, status, done/in-progress/blocked child counts
- Blockers section: any child with status "Blocked" or stale (72h+)
- Recent activity: children with `updated` in the last 24h (status change or new assignee)

**Detailed message** includes:
- Per-parent: full list of all children with status and assignee
- Recent comments from children updated in 24h
- Changelog entries (status transitions, reassignments) from the last 24h

### Slack Posting

1. Post condensed message to the configured channel → capture `ts` (message timestamp)
2. Store `ts` in `daily_update_log`
3. Post detailed message as a reply to `ts` (thread)

### Frontend (React + TypeScript)

New `DailyUpdateTab.tsx` component:
- Checks Jira connection status via `/api/connections` (provider key: `jira`)
- If not connected, shows connection prompt
- If connected, renders two panels: Tracked Tickets and Schedule Settings
- Polls nothing (settings are static until changed by user)
- "Post Now" button calls `POST /api/trigger-update`

---

## Error Handling

| Scenario | Behavior |
|----------|---------|
| Jira not connected | Cron skips posting, logs warning. UI shows "Jira not connected" banner. |
| Jira fetch fails for one ticket | Log error, skip that ticket, include a note in the Slack message |
| Slack post fails | Log error, return non-2xx to cron so it retries (up to 3x) |
| Already posted today | Cron returns 200 immediately (idempotency guard via `daily_update_log`) |
| No tracked tickets | Cron skips, logs "no tickets configured" |
| No channel configured | Cron skips, logs "no channel configured" |

---

## Out of Scope (v1)

- Confluence publishing (write daily update as a Confluence page)
- Per-ticket notification customization
- Multiple schedule configurations (only one post time globally)
- User-level auth (all users share one Jira connection)
- Historical update archive in the web UI
