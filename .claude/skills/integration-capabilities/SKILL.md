---
name: integration-capabilities
description: Load before building ANY feature that touches a third-party service. Defines what each integration can and cannot do with default scopes, how to request extra scopes, and scripted pushback for unsupported requests.
---

# Integration Capabilities Reference

## Supported Integrations

This template supports **only** these integrations:

- Google Sheets, Google Docs, Google Drive, Google Calendar, Gmail
- Slack (user OAuth)
- Jira, Confluence
- Anaheim (internal user directory — no OAuth, no scopes)

If a user requests any other integration (Notion, Airtable, HubSpot, Linear, GitHub, etc.), respond:

> "This template only supports [list above]. I can add the integration for you, but can't guarantee I'll integrate it correctly.

You do not have access to an LLM API.

---

## How API Calls Work

Every integration call goes through this chain:

```
Frontend fetch('/api/integration/{name}/...')
  → Go backend (dataapi.go) strips /api/integration, prepends /api/data
  → Data API at https://dataapi.{URL_BASE}/api/data/{name}/...
  → Upstream API with user's OAuth token injected automatically
```

---

## Frontend Integration Patterns

### 1. Checking Connection Status

Check if a user has connected an integration before fetching data:

```typescript
interface Connection {
  provider_config_key: string
  connection_id: string
}

const [connected, setConnected] = useState<boolean | null>(null)

useEffect(() => {
  fetch('/api/connections')
    .then(r => r.json())
    .then(data => {
      const connections: Connection[] = data.connections ?? []
      setConnected(connections.some(c => c.provider_config_key === '{providerKey}'))
    })
    .catch(() => setConnected(false))
}, [])

// Show "not connected" state
if (connected === false) {
  return (
    <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-6">
      <p className="text-yellow-800">Google Sheets not connected</p>
      <p className="text-yellow-700 text-sm">Connect your account using the integration bar above.</p>
    </div>
  )
}
```

**Provider keys differ from URL short names** — see each integration section below for the correct `provider_config_key`.

### 2. Starting an OAuth Flow

Trigger an OAuth connection from the frontend:

```typescript
const handleConnect = async () => {
  const res = await fetch('/api/connect/{shortName}', { method: 'POST' })
  if (!res.ok) {
    console.error('Failed to start OAuth flow')
    return
  }
  const { url } = await res.json()

  // Open OAuth popup
  const popup = window.open(url, '_blank', 'width=600,height=700')

  // Poll to detect when popup closes (user completed OAuth)
  const check = setInterval(() => {
    if (popup?.closed) {
      clearInterval(check)
      // Reload connection status
      fetch('/api/connections').then(/* refresh state */)
    }
  }, 500)
}
```

The user will complete OAuth in the popup, then the popup closes automatically. After it closes, re-fetch `/api/connections` to update the UI.

### 3. Extracting IDs from URLs

For Google resources (Sheets, Docs, Drive), users often paste full URLs instead of IDs. Extract the ID before making API calls:

```typescript
// Extract spreadsheet ID from Google Sheets URL
// https://docs.google.com/spreadsheets/d/{ID}/edit... → {ID}
function extractGoogleId(input: string): string | null {
  const trimmed = input.trim()

  // Match Google Docs/Sheets/Drive URL pattern
  const match = trimmed.match(/\/d\/([a-zA-Z0-9-_]+)/)
  if (match) return match[1]

  // Already an ID (no slashes)
  if (!/\//.test(trimmed)) return trimmed

  return null
}

// Usage
const id = extractGoogleId(userInput)
if (!id) {
  setError('Invalid URL or ID')
  return
}
const res = await fetch(`/api/integration/sheets/v4/spreadsheets/${id}/values/A1:Z200`)
```

Create `frontend/src/lib/googleId.ts`:
```typescript
export function extractGoogleId(input: string): string | null {
  const trimmed = input.trim()
  const match = trimmed.match(/\/d\/([a-zA-Z0-9-_]+)/)
  if (match) return match[1]
  if (!/\//.test(trimmed)) return trimmed
  return null
}
```

### 4. Error Handling Patterns

Handle common integration errors gracefully:

```typescript
const [loading, setLoading] = useState(false)
const [error, setError] = useState<string | null>(null)
const [data, setData] = useState<any>(null)

const fetchData = async () => {
  setLoading(true)
  setError(null)
  setData(null)

  try {
    const res = await fetch(`/api/integration/sheets/v4/spreadsheets/${id}/values/A1:Z200`)

    if (!res.ok) {
      if (res.status === 404) {
        setError('Spreadsheet not found. Check the URL and make sure you have access.')
      } else if (res.status === 403) {
        setError('Access denied. Make sure the file is shared with you.')
      } else {
        setError(`Request failed (${res.status}). Please try again.`)
      }
      return
    }

    const json = await res.json()
    setData(json)
  } catch (err) {
    setError('Network error — could not reach the server.')
  } finally {
    setLoading(false)
  }
}

// Display in UI
{loading && <p>Loading...</p>}
{error && <p className="text-red-600">{error}</p>}
{data && <div>/* render data */</div>}
```

### 5. Building Response State Handling

During development, the Go backend may be compiling when the frontend loads. Handle 503 responses gracefully:

```typescript
// frontend/src/lib/api.ts
export async function isBuildingResponse(res: Response): Promise<boolean> {
  if (res.status !== 503) return false
  const text = await res.text()
  return text.includes('compiling') || text.includes('building')
}

// Usage in component
const res = await fetch('/api/integration/sheets/...')
if (await isBuildingResponse(res)) {
  return <div className="bg-blue-50 p-4 rounded">Backend is compiling, please wait...</div>
}
```

### 6. Complete Component Example

Putting it all together — a Google Sheets viewer:

```typescript
import { useState, useEffect } from 'react'
import { extractGoogleId } from '../lib/googleId'

interface Connection {
  provider_config_key: string
}

function GoogleSheetsTab() {
  const [connected, setConnected] = useState<boolean | null>(null)
  const [input, setInput] = useState('')
  const [rows, setRows] = useState<string[][] | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/connections')
      .then(r => r.json())
      .then(data => {
        const connections: Connection[] = data.connections ?? []
        setConnected(connections.some(c => c.provider_config_key === 'google-sheet'))
      })
      .catch(() => setConnected(false))
  }, [])

  const handleLoad = async () => {
    const id = extractGoogleId(input)
    if (!id) {
      setError('Invalid URL or ID')
      return
    }

    setLoading(true)
    setError(null)
    setRows(null)

    try {
      const res = await fetch(`/api/integration/sheets/v4/spreadsheets/${id}/values/A1:Z200`)
      if (!res.ok) {
        setError(`Failed to load (${res.status})`)
        return
      }
      const data = await res.json()
      setRows(data.values ?? [])
    } catch {
      setError('Network error')
    } finally {
      setLoading(false)
    }
  }

  if (connected === false) {
    return <div>Not connected — use the integration bar to connect Google Sheets</div>
  }

  return (
    <div>
      <input
        type="text"
        value={input}
        onChange={e => setInput(e.target.value)}
        placeholder="Paste Google Sheets URL or ID"
      />
      <button onClick={handleLoad} disabled={loading || !input.trim()}>
        {loading ? 'Loading...' : 'Load'}
      </button>
      {error && <p className="text-red-600">{error}</p>}
      {rows && <table>{/* render table */}</table>}
    </div>
  )
}
```

---

## Integration Reference

Provider keys differ from the URL short names — see each integration below.

---

## Capability Matrix

### Google Sheets
**Default scope:** `spreadsheets` (read + write)
**Short name in URLs:** `sheets`
**Provider key (for connection check):** `google-sheet`

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| Read cell values and named ranges | Access files not owned by the user → needs `drive` scope |
| Write and append rows | Server-side formula execution (formulas run in Sheets, not via API) |
| Create new spreadsheets | Real-time push (polling only — no webhook/streaming) |
| Clear and update ranges | Batch copy between spreadsheets owned by different users |
| Read sheet metadata (tabs, dimensions) | |

**API examples:**
```
# Read values from a range
GET /api/integration/sheets/v4/spreadsheets/{spreadsheetId}/values/A1:Z200
→ { values: [["col1","col2"], ["val1","val2"]] }

# Update a range
PUT /api/integration/sheets/v4/spreadsheets/{spreadsheetId}/values/A1:Z200?valueInputOption=RAW
Body: { "values": [["col1","col2"],["val1","val2"]] }

# Append rows
POST /api/integration/sheets/v4/spreadsheets/{spreadsheetId}/values/A1:append?valueInputOption=RAW
Body: { "values": [["new row","data"]] }
```

> **Note:** Always include `/v4/` in the path. Omitting it will cause a 404 from Google.

---

### Google Docs
**Default scope:** `documents` (read + write)
**Short name in URLs:** `docs`
**Provider key (for connection check):** `google-docs`

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| Read document content and structure | Export to PDF/DOCX → needs `drive` scope |
| Create new documents | Access documents in shared drives not owned by user → needs `drive` scope |
| Insert, delete, and replace text via `batchUpdate` | Real-time collaboration / live cursors |
| Apply text formatting | Track changes or revision history |
| Insert inline images (by URI) | |

**API examples:**
```
# Get a document
GET /api/integration/docs/v1/documents/{documentId}
→ { title, body: { content: [{ paragraph: { elements: [{ textRun: { content } }] } }] } }

# Create a new document
POST /api/integration/docs/v1/documents
Body: { "title": "My Document" }

# Insert text via batchUpdate
POST /api/integration/docs/v1/documents/{documentId}:batchUpdate
Body: { "requests": [{ "insertText": { "location": { "index": 1 }, "text": "Hello world\n" } }] }
```

---

### Google Drive
**Default scope:** `drive.readonly`
**Short name in URLs:** `drive`
**Provider key (for connection check):** `google-drive`

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| List files and folders | Upload or create files → needs `drive` or `drive.file` scope |
| Read file metadata (name, type, owners) | Modify sharing permissions → needs `drive` scope |
| Download file content | Move or delete files → needs `drive` scope |
| Search files by name/type/parent | Access files created by other apps → needs `drive` scope (`drive.file` = app-created only) |

**Scope upgrade guidance:**
- Read-only use cases: keep `drive.readonly`
- App needs to create/upload files: request `drive.file` (least privilege)
- App needs full file management: request `drive` (broadest — flag to user)

**API examples:**
```
# List files
GET /api/integration/drive/v3/files?pageSize=20
→ { files: [{ id, name, mimeType, modifiedTime }] }

# Search files (q uses Drive query syntax)
GET /api/integration/drive/v3/files?q=name+contains+'report'+and+mimeType='application/vnd.google-apps.spreadsheet'

# Get file metadata
GET /api/integration/drive/v3/files/{fileId}?fields=id,name,mimeType,size,modifiedTime
```

---

### Google Calendar
**Default scope:** `calendar.events` (read + write events)
**Short name in URLs:** `calendar`
**Provider key (for connection check):** `google-calendar`

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| List events on any calendar the user owns | Create new calendars → needs `calendar` scope |
| Create, update, delete events | Modify calendar settings or sharing → needs `calendar` scope |
| Add attendees and set reminders | Access other users' calendars (only user's own) |
| Read recurring event instances | Read-only mode: request `calendar.events.readonly` instead |

**API examples:**
```
# List upcoming events (timeMin must be a URL-encoded ISO 8601 timestamp)
GET /api/integration/calendar/v3/calendars/primary/events?timeMin={now}&maxResults=10&orderBy=startTime&singleEvents=true
→ { items: [{ summary, start: { dateTime }, end: { dateTime }, location, htmlLink }] }

# Create an event
POST /api/integration/calendar/v3/calendars/primary/events
Body: { "summary": "Team Standup", "start": { "dateTime": "2026-03-10T09:00:00-07:00" }, "end": { "dateTime": "2026-03-10T09:30:00-07:00" } }

# Delete an event
DELETE /api/integration/calendar/v3/calendars/primary/events/{eventId}
```

> **Note:** Always include `/v3/` in the path. Omitting it will cause a 404 from Google.

---

### Gmail
**Default scope:** `gmail.readonly`
**Short name in URLs:** `google-mail`
**Provider key (for connection check):** `google-mail`

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| Read messages and threads | Send emails → needs `gmail.send` scope |
| List and search labels | Create or update drafts → needs `gmail.compose` scope |
| Search emails by query | Delete messages or modify labels → needs `gmail.modify` scope |
| Read message headers and body | |

**Gmail is read-only by default.** If a user asks to send email, reply, or draft — flag that this requires the `gmail.send` or `gmail.compose` scope and the user will need to reconnect.

**API examples:**
```
# List messages — returns IDs only, not content
GET /api/integration/google-mail/gmail/v1/users/me/messages?maxResults=5&labelIds=INBOX
→ { messages: [{ id, threadId }] }

# Get message headers only (efficient — avoids downloading full body)
GET /api/integration/google-mail/gmail/v1/users/me/messages/{id}?format=metadata&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Date
→ { id, payload: { headers: [{ name, value }] } }

# Get full message including body
GET /api/integration/google-mail/gmail/v1/users/me/messages/{id}?format=full
→ { id, snippet, payload: { headers, parts } }

# List labels
GET /api/integration/google-mail/gmail/v1/users/me/labels
→ { labels: [{ id, name, type }] }
```

> **Pattern:** The list endpoint only returns IDs. To show subjects/senders, fetch each message with `format=metadata`. Use `Promise.all()` to fetch in parallel.

---

### Slack (User OAuth)
**Default scopes:** `users:read`, `channels:read`, `groups:read`
**Short name in URLs:** `slack`
**Provider key (for connection check):** `slack`

> Slack uses `user_scopes`, not `scopes`. Pass via `user_scopes` when starting OAuth.

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| List workspace members | Read message history → needs `channels:history` (public) or `groups:history` (private) |
| List public channels | Post messages → needs `chat:write` |
| List private channels the user is in | Search messages → needs `search:read` |
| Get basic user identity | Star/unstar messages → needs `stars:write` |
| | Read starred items → needs `stars:read` |

**Important:** The bot's Slack token (used in `main.go` via `slacklib`) is separate from the user OAuth token. User OAuth is for acting on behalf of the authenticated user — not for bot actions.

**API examples:**
```
# List channels
GET /api/integration/slack/conversations.list?types=public_channel&limit=50
→ { ok, channels: [{ id, name, is_private }] }

# Get channel message history (requires channels:history scope)
GET /api/integration/slack/conversations.history?channel={channelId}&limit=20
→ { ok, messages: [{ ts, text, user }] }

# Get user identity
GET /api/integration/slack/users.identity
→ { ok, user: { id, name, email }, team: { id } }

# Search messages (requires search:read scope)
GET /api/integration/slack/search.messages?query=keyword
→ { ok, messages: { matches: [{ text, channel, ts }] } }
```

> **Note:** Slack uses dot-notation method paths (e.g. `conversations.list`), not REST-style paths. No version prefix needed.

---

### Jira
**Default scopes:** `read:jira-work`, `write:jira-work`, `read:jira-user`
**Short name in URLs:** `jira`
**Provider key (for connection check):** `jira`

> Jira requests are automatically prefixed with the Applied Intuition cloud ID — use standard API paths.

> **⚠️ BREAKING:** `/rest/api/3/search` has been removed (returns 410). Always use `/rest/api/3/search/jql` instead.

> **⚠️ WARNING:** JQL queries must include at least one filter — bare `ORDER BY` with no restriction returns 400. Always include a filter like `assignee = currentUser()` or `project = MYPROJECT`.

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| Read projects and issues | Admin operations (project config, field schemes) |
| Search issues via JQL | Manage workflows or issue types |
| Create and update issues | Delete projects |
| Post comments | Bulk operations across all projects not visible to user |
| Read worklogs | |
| View user profiles | |

**API examples:**
```
# Search issues via JQL — MUST use /search/jql (NOT /search, which returns 410)
# MUST include at least one filter — bare ORDER BY with no restriction returns 400
GET /api/integration/jira/rest/api/3/search/jql?jql=assignee+%3D+currentUser()+ORDER+BY+updated+DESC&maxResults=5&fields=summary,status,priority,assignee,updated
→ { issues: [{ id, key, fields: { summary, status: { name }, priority: { name }, assignee, updated } }] }

# Other valid bounded JQL examples:
#   project = MYPROJECT ORDER BY updated DESC
#   assignee = currentUser() AND status != Done ORDER BY updated DESC

# Get a specific issue
GET /api/integration/jira/rest/api/3/issue/{issueKey}
→ { id, key, fields: { summary, description, status, priority, assignee } }

# Create an issue
POST /api/integration/jira/rest/api/3/issue
Body: { "fields": { "project": { "key": "MYPROJECT" }, "summary": "Bug title", "issuetype": { "name": "Bug" } } }

# Add a comment
POST /api/integration/jira/rest/api/3/issue/{issueKey}/comment
Body: { "body": { "type": "doc", "version": 1, "content": [{ "type": "paragraph", "content": [{ "type": "text", "text": "Comment text" }] }] } }
```

> **Two hard rules:**
> 1. Always use `/rest/api/3/search/jql` — `/rest/api/3/search` is removed (410)
> 2. Always include a filter in JQL (`assignee = currentUser()`, `project = X`, etc.) — unbounded queries return 400

---

### Confluence
**Default scopes:** `read:space:confluence`, `read:page:confluence`, `write:page:confluence`
**Short name in URLs:** `confluence`
**Provider key (for connection check):** `confluence`

> Uses granular scopes (v1 API deprecated). Requests auto-prefixed with Applied cloud ID.

> **⚠️ CRITICAL:** The default scopes are **v2 granular scopes**. This means you **MUST use the v2 API** (`/wiki/api/v2/`). The v1 API (`/wiki/rest/api/content`) will return **401 Unauthorized** even though the user is connected. Do not use v1 paths.

| Can Do (default scope) | Cannot Do / Needs Extra Scope |
|------------------------|-------------------------------|
| List and read spaces | Read blog posts → needs `read:blogpost:confluence` |
| Read pages | Write blog posts → needs `write:blogpost:confluence` |
| Create and update pages | Read comments → needs `read:comment:confluence` |
| | Write comments → needs `write:comment:confluence` |
| | Read attachments → needs `read:attachment:confluence` |
| | Search content metadata → needs `read:content.metadata:confluence` |
| | Manage space permissions |

**API examples — use /wiki/api/v2/ paths (NOT /wiki/rest/api/ which returns 401):**
```
# List pages (sorted by most recently modified)
GET /api/integration/confluence/wiki/api/v2/pages?limit=5&sort=-modified-date
→ { results: [{ id, title, spaceId, version: { createdAt }, _links: { editui, webui } }] }

# Get a specific page with body content
GET /api/integration/confluence/wiki/api/v2/pages/{pageId}?body-format=storage
→ { id, title, body: { storage: { value: "<html>" } } }

# List spaces
GET /api/integration/confluence/wiki/api/v2/spaces?limit=10
→ { results: [{ id, key, name, type }] }

# Create a page
POST /api/integration/confluence/wiki/api/v2/pages
Body: { "spaceId": "SPACEID", "title": "My Page", "body": { "representation": "storage", "value": "<p>Hello</p>" } }

# Update a page (version number required)
PUT /api/integration/confluence/wiki/api/v2/pages/{pageId}
Body: { "id": "PAGEID", "title": "Updated Title", "version": { "number": 2 }, "body": { "representation": "storage", "value": "<p>Updated</p>" } }
```

---

### Anaheim (Internal User Directory)
**Auth:** Google OAuth ID token (service account) — no user OAuth, no scopes
**Access:** Available to all apps via the vendored `anaheim` client in `go.apps.applied.dev/lib/anaheim`

Anaheim is Applied's internal employee directory. It is **not** a user-connected integration — the app itself authenticates with a service account. Use it server-side in Go handlers, not from the frontend.

| Can Do | Cannot Do |
|--------|-----------|
| Look up any employee by email | Write or modify employee records |
| Filter employees by team, title, manager, GitHub name | Access data outside Applied's org |
| Get a user's current or historical managers | Real-time push/webhooks |
| Get the full manager chain up to the CEO | |
| Get direct and all-reports for a manager | |

**Available fields on each employee:** `firstName`, `lastName`, `email`, `teamName`, `title`, `office`, `timezone`, `managerEmail`, `githubName`, `slackId`, `linkedinUrl`, `joinDate`, `jobFamily`, `profileImageUrl`, `isActive`, `memberType`

**API examples (HTTP):**
```
# Get a single user by email
GET https://anaheim.applied.co/applied/api/v1/user/{email}
→ { firstName, lastName, email, teamName, title, slackId, githubName, jobFamily, ... }

# Filter/list users (all filters are optional; AND across fields, OR within a field's list)
POST https://anaheim.applied.co/applied/api/v1/user
Body: { "teams": ["Anaheim"], "titles": ["Manager", "Software Engineer"], "manager_emails": ["mgr@applied.co"] }
→ { users: [...] }

# Get current/historical managers of a user
POST https://anaheim.applied.co/applied/api/v1/reporting_data/managers
Body: { "report_email": "user@applied.co", "get_current": true }
→ ["manager@applied.co"]

# Get full manager chain (ordered, bottom → top)
POST https://anaheim.applied.co/applied/api/v1/reporting_data/manager_chain
Body: { "report_email": "user@applied.co" }
→ ["manager@applied.co", "vp@applied.co", "ceo@applied.co"]

# Get direct reports
POST https://anaheim.applied.co/applied/api/v1/reporting_data/direct_reports
Body: { "manager_email": "mgr@applied.co", "get_current": true }
→ ["report1@applied.co", "report2@applied.co"]

# Get all reports (including skip-level)
POST https://anaheim.applied.co/applied/api/v1/reporting_data/all_reports
Body: { "manager_email": "mgr@applied.co" }
→ ["report1@applied.co", "report2@applied.co", "skip@applied.co"]
```

**Go usage (preferred — use the vendored client, not raw HTTP):**
```go
// anaheimClient is initialized in main.go and passed to registerAPIRoutes
employees, err := anaheimClient.GetEmployeesByFilter(ctx, anaheim.UserFilter{
    Teams:  []string{"Engineering"},
    Titles: []string{"Software Engineer"},
})
```

> See `.claude/docs/blueprints/static/anaheim/anaheim.md` for the full Go client implementation.

#### Frontend API Endpoints (Backend Required)

To expose Anaheim to the frontend, add these Go handlers in `api.go`:

```go
// GET /api/anaheim/user/:email — single user lookup
func handleAnaheimGetUser(client *anaheim.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        email := c.Param("email")
        user, err := client.GetEmployee(c.Request.Context(), email)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
            return
        }
        c.JSON(http.StatusOK, gin.H{"user": user})
    }
}

// POST /api/anaheim/users — search by query string (frontend convenience wrapper)
// Body: { "query": "John Doe" or "john@applied.co" }
func handleAnaheimSearchUsers(client *anaheim.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            Query string `json:"query"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
            return
        }

        // Try email lookup first
        if strings.Contains(req.Query, "@") {
            user, err := client.GetEmployee(c.Request.Context(), req.Query)
            if err == nil {
                c.JSON(http.StatusOK, gin.H{"users": []interface{}{user}})
                return
            }
        }

        // Otherwise, filter by name pattern (simplified — adjust as needed)
        users, err := client.GetAllEmployees(c.Request.Context())
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
            return
        }

        // Filter by name match
        query := strings.ToLower(req.Query)
        var results []anaheim.Employee
        for _, u := range users {
            name := strings.ToLower(u.FirstName + " " + u.LastName)
            if strings.Contains(name, query) || strings.Contains(strings.ToLower(u.Email), query) {
                results = append(results, u)
            }
        }

        c.JSON(http.StatusOK, gin.H{"users": results})
    }
}
```

Register these in `registerAPIRoutes`:
```go
if anaheimClient != nil {
    api.GET("/anaheim/user/:email", handleAnaheimGetUser(anaheimClient))
    api.POST("/anaheim/users", handleAnaheimSearchUsers(anaheimClient))
}
```

#### Frontend Component Example

```typescript
interface Employee {
  firstName: string
  lastName: string
  email: string
  teamName: string
  title: string
  profileImageUrl: string
  managerEmail: string
}

function AnaheimDirectory() {
  const [employees, setEmployees] = useState<Employee[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSearch = async () => {
    if (!searchQuery.trim()) return
    setLoading(true)

    try {
      const res = await fetch('/api/anaheim/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: searchQuery }),
      })

      if (!res.ok) throw new Error('Search failed')
      const data = await res.json()
      setEmployees(data.users || [])
    } catch {
      console.error('Failed to search')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <input
        type="text"
        value={searchQuery}
        onChange={e => setSearchQuery(e.target.value)}
        onKeyPress={e => e.key === 'Enter' && handleSearch()}
        placeholder="Search by name or email"
      />
      <button onClick={handleSearch} disabled={loading}>
        {loading ? 'Searching...' : 'Search'}
      </button>

      <div className="grid gap-4">
        {employees.map(emp => (
          <div key={emp.email} className="border rounded p-4">
            {emp.profileImageUrl ? (
              <img src={emp.profileImageUrl} alt={emp.firstName} className="w-16 h-16 rounded-full" />
            ) : (
              <div className="w-16 h-16 rounded-full bg-blue-500 text-white flex items-center justify-center">
                {emp.firstName[0]}{emp.lastName[0]}
              </div>
            )}
            <h4>{emp.firstName} {emp.lastName}</h4>
            <p>{emp.title}</p>
            <p>{emp.email}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
```

---

## Requesting Extra Scopes

For Google, Jira, Confluence — pass via `scopes`:
```
POST /api/data/oauth/start?integration=sheets&scopes=https://www.googleapis.com/auth/drive
```

For Slack — pass via `user_scopes`:
```
POST /api/data/oauth/start?integration=slack&user_scopes=channels:history,chat:write
```

**If the user has already connected the integration, they must reconnect for new scopes to take effect.** Always warn the user before requesting additional scopes.

---

## Pushback Decision Tree

When a user requests a feature involving an integration:

1. **Is the integration supported?**
   - No → "This template only supports [list]. Would you like to achieve this with one of those instead?"

2. **Is the feature within the default scope?**
   - Yes → proceed
   - No, but achievable with an extra scope → **stop and push back** (see below)
   - No, fundamentally impossible (e.g. server-side formula execution, real-time streaming) → "This isn't possible via the API — [explain why]. Here's what we can do instead: [alternative]."

3. **Does the feature require acting as the bot vs. the user?**
   - Bot actions (send Slack message as bot, respond to commands) → use `slacklib` in `main.go`, no user OAuth needed
   - User actions (post as user, read user's calendar) → requires user OAuth flow
   - Flag this distinction early — it affects the architecture.

---

## Out-of-Scope Permission Pushback

**Do NOT build the feature.** When a request requires scopes beyond the defaults, tell the user what permission is needed and how to request it — then wait for confirmation before proceeding.

**Script:**
> "This feature requires the `{scope}` permission, which isn't granted by default. Users will need to reconnect their {integration} account to authorize it.
>
> To enable this, you'll need to request the additional scope — reach out to the Apps Platform team in **#eng-apps-platform-v2** to get it added.
>
> Once the scope is approved and added, I can build this feature. Want me to proceed assuming that permission will be granted, or would you like to explore an alternative that works within the current defaults?"

**Examples of what triggers this pushback:**

| User asks for... | Integration | Missing scope | Pushback trigger |
|------------------|-------------|---------------|------------------|
| Send email on behalf of user | Gmail | `gmail.send` | Default is `gmail.readonly` |
| Draft or reply to emails | Gmail | `gmail.compose` | Default is `gmail.readonly` |
| Post a Slack message as the user | Slack | `chat:write` | Not in default user scopes |
| Read Slack message history | Slack | `channels:history` / `groups:history` | Not in default user scopes |
| Create a new Google Calendar | Google Calendar | `calendar` | Default is `calendar.events` only |
| Upload files to Google Drive | Google Drive | `drive.file` or `drive` | Default is `drive.readonly` |
| Write blog posts in Confluence | Confluence | `write:blogpost:confluence` | Not in default scopes |

**Never silently build around a missing scope** (e.g. do not fake sending an email by logging it). Always surface the scope gap to the user explicitly.
