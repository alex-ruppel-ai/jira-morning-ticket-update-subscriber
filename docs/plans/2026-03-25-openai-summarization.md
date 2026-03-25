# OpenAI Summarization — Design & Implementation Plan

**Date:** 2026-03-25
**Status:** Ready to implement

---

## Overview

Replace the Anthropic SDK in `summarize.go` with OpenAI's Chat Completions API. The app already collects all Jira data (parent tickets, children, assignees, comments, changelogs from the last 24h) — this plan wires that data into ChatGPT to produce readable, narrative Slack summaries instead of raw template output.

---

## What Changes

### Files modified

| File | Change |
|------|--------|
| `summarize.go` | Replace Anthropic client + calls with OpenAI `go-openai` client |
| `go.mod` / `go.sum` | Remove `anthropic-sdk-go`, add `go-openai` |

### Files unchanged
Everything else stays the same: `db.go`, `jira_update.go`, `api.go`, `main.go`, frontend. The summarization is a drop-in swap — `summarizeUpdate(ctx, updates)` returns the same `(condensed, details, error)` tuple regardless of which model is under the hood.

---

## Model Choice

**`gpt-4o-mini`** — default. Fast, cheap (~$0.15/1M input tokens), and more than capable for summarizing structured Jira data into prose.

**`gpt-4o`** — fallback option if the user wants higher quality. Costs ~10x more but produces noticeably better summaries for complex tickets.

---

## OpenAI Go SDK

Library: `github.com/sashabaranov/go-openai` — the standard community Go SDK, used by most Go projects.

```go
import openai "github.com/sashabaranov/go-openai"

client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model: openai.GPT4oMini,
    Messages: []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
        {Role: openai.ChatMessageRoleUser, Content: userPrompt},
    },
    MaxTokens: 1024,
})
text := resp.Choices[0].Message.Content
```

---

## Prompts

### Condensed message (main Slack post)

**System:** You are an engineering team assistant. You write concise, informative daily status updates for a Slack channel. Be direct. Synthesize — don't just list. Use Slack markdown (*bold*, `code`).

**User:**
```
Today is {date}. Generate a daily Jira status update for Slack.

{serialized Jira data for all parent tickets}

Requirements:
- Start with "*Daily Update — {Mon Jan 2}*"
- 5–10 lines total
- Highlight blockers and stale tickets (no update in 3+ days)
- Summarize last-24h activity (status changes, new comments, reassignments)
- Do NOT list every child ticket individually — synthesize the state
```

### Detailed message (thread reply, one per parent)

**System:** Same as above.

**User:**
```
Generate a detailed Slack thread reply for one parent Jira ticket.

{serialized data for this parent + all children + comments + changelog}

Requirements:
- Start with "*Full breakdown — {TICKET-KEY}*"
- 1–2 sentence narrative on overall state
- List each child ticket: `KEY` Title [Status] @assignee
- Summarize any recent comments (what was said, not just who)
- List status/assignment changes from the last 24h
```

---

## Secret Setup

Set the OpenAI API key as a secret before deploying:

```bash
apps-platform app secret set openai-api-key sk-...YOUR-KEY...
```

The app reads it via `os.Getenv("OPENAI_API_KEY")`. The secret name maps to env var `OPENAI_API_KEY` automatically via the apps-platform secret injection.

---

## Graceful Fallback

If `OPENAI_API_KEY` is not set or the API call fails, `summarizeUpdate` returns an error and `runDailyUpdate` falls back to the template-based `buildCondensedMessage` / `buildDetailedMessage` functions. The app always posts something — it never silently fails.

---

## Implementation Tasks

### Task 1: Swap dependencies in go.mod
```bash
go get github.com/sashabaranov/go-openai
# optionally: go mod edit -droprequire github.com/anthropics/anthropic-sdk-go
```

### Task 2: Rewrite `summarize.go`
- Import `go-openai` instead of `anthropic-sdk-go`
- Change env var from `ANTHROPIC_API_KEY` to `OPENAI_API_KEY`
- Replace `anthropic.NewClient` → `openai.NewClient`
- Replace `client.Messages.New(...)` → `client.CreateChatCompletion(...)`
- Use `resp.Choices[0].Message.Content` for the text
- Keep the same function signatures: `summarizeUpdate`, `generateCondensedSummary`, `generateDetailedSummary`

### Task 3: Build and verify
```bash
go build ./...
```

### Task 4: Set secret and deploy
```bash
apps-platform app secret set openai-api-key sk-YOUR-KEY
apps-platform app deploy
```

### Task 5: Test via "Post Now"
Click Post Now in the UI. Check `#apps-platform-slack-test` for a narrative summary. Confirm logs show `[Summarize] condensed summary generated` and `[Summarize] detailed summary for ... generated`.

---

## Out of Scope

- Streaming (not needed for background cron jobs)
- Function calling / structured outputs (plain text is fine for Slack)
- Model selection in the UI (can be added later)
- Per-ticket prompt customization
