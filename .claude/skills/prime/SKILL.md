---
description: Prime your understanding of this Slack bot template codebase before starting work
---

# Prime

Execute the `Workflow` section to orient yourself in the codebase, then produce the `Report`.

## Workflow

1. Read `main.go` — Slack event handler registration (`OnMention`, `OnDM`, `Command`, `ViewSubmission`, `Action`) and server wiring
2. Read `api.go` — all REST API routes under `/api/` (send-message, send-dm, members, events, feedback, anaheim)
3. Read `frontend/src/App.tsx` — React tab structure, `StatusContext`, and component routing

## Report

Summarize:
- Which Slack event handlers exist and what they do
- Which API routes are registered and their request/response shapes
- Which frontend tabs exist and which component each maps to
- Any integration boundaries relevant to the current task (Slack events ↔ eventlog ↔ API ↔ React)
