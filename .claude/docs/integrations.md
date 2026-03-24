# Data API Integration Guide

## How to Call Integration Endpoints

Use the **short integration name** (not the OAuth provider key) in API URL paths:

```
GET /api/integration/{short-name}/...
```

## Integration Name Reference

| Integration     | Short Name (use in URLs) | OAuth Provider Key (do NOT use in URLs) |
|-----------------|--------------------------|------------------------------------------|
| Google Sheets   | `sheets`                 | `google-sheet`                           |
| Google Docs     | `docs`                   | `google-docs`                            |
| Google Drive    | `drive`                  | `google-drive`                           |
| Google Calendar | `calendar`               | `google-calendar`                        |
| Gmail           | `google-mail`            | `google-mail`                            |
| Jira            | `jira`                   | `jira`                                   |
| Confluence      | `confluence`             | `confluence`                             |
| Slack (User)    | `slack`                  | `slack-user`                             |

## Common Mistake

The OAuth provider key (returned in `provider_config_key` from `/api/connections`) is **not** the same as the short name used in API URLs.

**Wrong:**
```
GET /api/integration/google-sheet/spreadsheets/...
GET /api/integration/google-calendar/events
```

**Correct:**
```
GET /api/integration/sheets/v4/spreadsheets/...
GET /api/integration/calendar/v3/calendars/primary/events
```

## Example: Google Sheets

```
GET /api/integration/sheets/v4/spreadsheets/{spreadsheetId}/values/{range}

POST /api/integration/sheets/v4/spreadsheets/{spreadsheetId}/values/{range}:append?valueInputOption=RAW
Body: { "values": [["col1","col2"]] }
```

## Example: Google Calendar

```
GET /api/integration/calendar/v3/calendars/primary/events?timeMin={iso8601}&maxResults=10&orderBy=startTime&singleEvents=true

POST /api/integration/calendar/v3/calendars/primary/events
Body: { "summary": "Meeting", "start": { "dateTime": "..." }, "end": { "dateTime": "..." } }
```

## Example: Google Docs

```
GET /api/integration/docs/v1/documents/{documentId}

POST /api/integration/docs/v1/documents
Body: { "title": "My Doc" }
```

## Example: Google Drive

```
GET /api/integration/drive/v3/files?pageSize=20

GET /api/integration/drive/v3/files?q=name+contains+'report'
```

## Example: Gmail

```
GET /api/integration/google-mail/gmail/v1/users/me/messages?q=is:unread&maxResults=10

GET /api/integration/google-mail/gmail/v1/users/me/messages/{messageId}?format=full

GET /api/integration/google-mail/gmail/v1/users/me/labels
```

## Example: Jira

```
GET /api/integration/jira/rest/api/3/search?jql=project=MYPROJECT

POST /api/integration/jira/rest/api/3/issue
Body: { "fields": { "project": { "key": "MYPROJECT" }, "summary": "Bug", "issuetype": { "name": "Bug" } } }
```

## Example: Confluence

```
GET /api/integration/confluence/wiki/rest/api/content?spaceKey=MYSPACE

POST /api/integration/confluence/wiki/rest/api/content
Body: { "type": "page", "title": "My Page", "space": { "key": "MYSPACE" }, "body": { "storage": { "value": "<p>Hello</p>", "representation": "storage" } } }
```

## Example: Slack User

```
GET /api/integration/slack/conversations.list

GET /api/integration/slack/conversations.history?channel=C123ABC

GET /api/integration/slack/search.messages?query=hello

POST /api/integration/slack/chat.postMessage
Body: { "channel": "C123ABC", "text": "Hello from the app!" }

GET /api/integration/slack/users.identity
```
