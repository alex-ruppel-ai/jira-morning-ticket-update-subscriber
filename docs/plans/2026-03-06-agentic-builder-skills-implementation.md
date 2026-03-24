# Agentic App Builder Skills Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `v1-scoping` and `user-journey` skills to the agentic-apps-builder-template, plus a CLAUDE.md that enforces the full workflow chain.

**Architecture:** Three files under `agentic-apps-builder-template/.claude/` — a CLAUDE.md for workflow enforcement and two skill SKILL.md files. Each skill cross-references its neighbours so the chain is self-reinforcing. No code is generated; these are process documentation files for Claude Code.

**Tech Stack:** Markdown, Claude Code skills format (YAML frontmatter + Markdown body)

---

### Task 1: Create the .claude directory structure

**Files:**
- Create: `agentic-apps-builder-template/.claude/CLAUDE.md`
- Create: `agentic-apps-builder-template/.claude/skills/v1-scoping/SKILL.md`
- Create: `agentic-apps-builder-template/.claude/skills/user-journey/SKILL.md`

**Step 1: Create directory structure**

```bash
mkdir -p agentic-apps-builder-template/.claude/skills/v1-scoping
mkdir -p agentic-apps-builder-template/.claude/skills/user-journey
```

Run from: `/workspace/apps-platform-example-apps`
Expected: no output, directories created

**Step 2: Verify directories exist**

```bash
find agentic-apps-builder-template/.claude -type d
```

Expected output:
```
agentic-apps-builder-template/.claude
agentic-apps-builder-template/.claude/skills
agentic-apps-builder-template/.claude/skills/v1-scoping
agentic-apps-builder-template/.claude/skills/user-journey
```

---

### Task 2: Write CLAUDE.md

**Files:**
- Create: `agentic-apps-builder-template/.claude/CLAUDE.md`

**Step 1: Write the file**

Create `agentic-apps-builder-template/.claude/CLAUDE.md` with this exact content:

```markdown
# Agentic App Builder Template

This template is used to build and deploy web apps on Applied's Apps Platform (Go + React, Cloud Run).

## Required Workflow for New Features or Apps

When a user asks you to build something new, you MUST follow this workflow in order. Do not skip steps.

1. **Invoke `v1-scoping` (phase 1)** — Assess feasibility and set a rough v1 boundary BEFORE brainstorming
2. **Invoke `brainstorming`** — Design the full solution within the confirmed scope
3. **Invoke `v1-scoping` (phase 2)** — Cut the brainstormed design to the smallest deployable v1
4. **Invoke `user-journey`** — Walk through exactly how a user interacts with the v1
5. **Invoke `writing-plans`** — Produce a bite-sized implementation plan

## Why This Order Matters

- Phase 1 of v1-scoping catches infeasible requests before wasting time designing them
- Brainstorming runs on a confirmed, feasible scope — not wishful thinking
- Phase 2 of v1-scoping prunes the full design down to what can actually be shipped and tested
- User journey validates the pruned design is coherent from a real user's perspective
- writing-plans converts the validated design into actionable tasks

## Platform Context

- Apps deploy to Google Cloud Run via `apps-platform app deploy`
- Go backend with embedded React frontend (via `go:embed`)
- See `docs/ai/` for architecture patterns, database connections, secrets, and deployment
- See `AGENTS.md` for quick reference on local dev and deploy commands
```

**Step 2: Verify file was written**

```bash
cat agentic-apps-builder-template/.claude/CLAUDE.md
```

Expected: full file content displays

**Step 3: Commit**

```bash
git add agentic-apps-builder-template/.claude/CLAUDE.md
git commit -m "feat: add CLAUDE.md workflow enforcement for agentic app builder template"
```

---

### Task 3: Write v1-scoping skill

**Files:**
- Create: `agentic-apps-builder-template/.claude/skills/v1-scoping/SKILL.md`

**Step 1: Write the file**

Create `agentic-apps-builder-template/.claude/skills/v1-scoping/SKILL.md` with this exact content:

````markdown
---
name: v1-scoping
description: Use when starting a new feature or app build — before brainstorming to assess feasibility and after brainstorming to cut the design to the smallest deployable v1. Also use standalone when scope creep occurs mid-implementation.
---

# v1-scoping

## Overview

Forces two explicit scoping checkpoints around brainstorming: a feasibility check before (so you don't design something that can't be built) and a v1 filter after (so you don't build more than what's needed to start iterating).

**REQUIRED BACKGROUND:** This skill is invoked twice in the workflow — once before `brainstorming` and once after. The phase is determined by context.

## Phase 1: Feasibility Check (before brainstorming)

Ask these three questions **one at a time**. Wait for the user's answer before asking the next.

1. **What does success look like?** Ask for one sentence. This anchors scope and prevents gold-plating.

2. **What external dependencies does this require?** Listen for: third-party APIs, real-time data feeds, OAuth providers, payment processors, ML models, websockets. These are the most common sources of infeasible requests.

3. **What's the hardest part to build?** This surfaces hidden complexity before the design begins.

### After the three questions

Based on the answers, do one of:

**If feasible:**
- Confirm it out loud: "This is buildable. Here's what v1 looks like: [one sentence rough boundary]"
- Example: "v1 = user submits a form, data saves to Postgres, confirmation page shown. No auth, no email, no real-time."
- **REQUIRED NEXT:** Invoke `brainstorming`

**If partially infeasible:**
- Name the specific blocker: "X isn't possible because Y"
- Propose what CAN be built: "Instead, we can Z"
- Get user agreement, then set the rough v1 boundary
- **REQUIRED NEXT:** Invoke `brainstorming`

**If completely infeasible:**
- Say so clearly and explain why
- Propose an alternative direction if possible
- Do NOT proceed to brainstorming until the user agrees on a feasible direction

## Phase 2: v1 Filter (after brainstorming)

Take the full brainstormed design and apply the v1 filter. The output is a 3-column table.

**For each feature in the design, ask:**
- Can this be hardcoded or faked in v1?
- Does v1 work without this?
- Does cutting this still leave something deployable and testable by a real user?

**Output format:**

| In v1 | Deferred to v2 | Cut entirely |
|-------|----------------|--------------|
| [feature] | [feature] | [feature] |

**Rules for the table:**
- "In v1" = must be present for a user to complete the core loop
- "Deferred to v2" = valuable but not required for the first test
- "Cut entirely" = nice-to-have that adds complexity without proportionate value

Present the table to the user. Get their sign-off. Adjust if they disagree.

**REQUIRED NEXT:** Invoke `user-journey`

## Standalone Use (scope creep)

If invoked mid-implementation when scope is expanding, run Phase 2 only against the current feature list. Output the same 3-column table.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Putting auth in v1 when the app works without it | Defer auth to v2 unless it's the core feature |
| Treating "nice to have" as "required" | If a user can complete the core loop without it, defer it |
| Skipping feasibility because the request sounds simple | Simple-sounding requests often have hidden external dependencies |
| Moving to brainstorming before user agrees on rough v1 boundary | Always get explicit agreement before proceeding |
````

**Step 2: Verify file was written**

```bash
wc -l agentic-apps-builder-template/.claude/skills/v1-scoping/SKILL.md
```

Expected: ~80 lines

**Step 3: Commit**

```bash
git add agentic-apps-builder-template/.claude/skills/v1-scoping/SKILL.md
git commit -m "feat: add v1-scoping skill to agentic app builder template"
```

---

### Task 4: Write user-journey skill

**Files:**
- Create: `agentic-apps-builder-template/.claude/skills/user-journey/SKILL.md`

**Step 1: Write the file**

Create `agentic-apps-builder-template/.claude/skills/user-journey/SKILL.md` with this exact content:

````markdown
---
name: user-journey
description: Use after v1 is scoped to walk through how a real user interacts with the app — what they click, what the app shows, and how data is pulled or pushed at a high level. Collaborative: agent drafts each section, user corrects before moving on.
---

# user-journey

## Overview

Validates the scoped v1 design by walking through it as a real user would experience it. Catches design gaps (missing screens, unclear data sources, broken flows) before implementation begins.

**REQUIRED BACKGROUND:** Run this after `v1-scoping` phase 2 has produced the In/Deferred/Cut table. Only walk through what's in v1.

## Process

Work through four sections **one at a time**. Draft each section based on the design, present it to the user, wait for corrections, then move on.

Do NOT present all four sections at once.

### Section 1: Entry Point

Draft a short description covering:
- How does the user get to the app? (direct URL, link from Slack, internal tool, etc.)
- What do they see when they first arrive? (landing page copy, login screen, empty state, loading spinner)
- What action is available to them immediately?

Present it. Ask: "Does this match what you have in mind? Anything different?"

---

### Section 2: Core Interaction Loop

For each of the 2-4 key actions a user takes in v1, describe:

**Format per action:**
```
Action: [what the user does — e.g., "clicks Submit on the feedback form"]
Sees: [what the UI shows before the action]
Result: [what happens after — keep it high level, e.g., "form clears, success banner appears"]
Backend: [one sentence on what the app does — e.g., "POST /api/feedback saves to Postgres"]
```

Keep backend descriptions high level. The goal is to confirm the interaction makes sense, not to spec the API.

Present all actions for this section together. Ask: "Does this capture the full interaction loop? Missing anything?"

---

### Section 3: Data Flows

For each user action from Section 2, describe the data movement:

**Format:**
```
[User action] → [API endpoint or function] → [data source] → [response to user]
```

Example:
```
User submits form → POST /api/submit → writes to Postgres → returns {success: true} → UI shows confirmation
User loads page → GET /api/items → reads from Postgres → returns item list → UI renders list
```

Keep it one line per flow. No need for field-level detail — just source, destination, and what the user sees.

Present the flows. Ask: "Does this data model make sense? Any flows missing or wrong?"

---

### Section 4: Edge Cases

Draft what happens in these situations (only include ones relevant to the v1):

- **Empty state** — user arrives with no data yet
- **Error state** — backend call fails or returns an error
- **Loading state** — async operation in progress
- **Invalid input** — user submits a form with bad data

Keep each to one sentence: "If X happens, the user sees Y."

Present the edge cases. Ask: "Anything else that could go wrong in v1 that we should account for?"

---

### Closing Summary

After all four sections are approved, write a single paragraph in plain English describing the full journey. Write it as if explaining the app to a non-technical stakeholder.

Example:
> "A user opens the app and sees a list of submitted feedback items. They click 'New Feedback', fill in a category and description, and submit. The item appears in the list immediately. If the submission fails, an error message appears and their input is preserved so they can retry."

Present the summary. Ask: "Does this accurately describe what we're building?"

**REQUIRED NEXT:** Invoke `writing-plans`

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Describing v2 features in the journey | Only walk through what's in the v1 In column |
| Skipping edge cases because "it's just v1" | Empty states and error states must exist in v1 or the app feels broken |
| Presenting all sections at once | Go one section at a time — corrections in section 1 affect section 2 |
| Vague backend descriptions ("does stuff with the data") | Name the endpoint and data source, even at high level |
````

**Step 2: Verify file was written**

```bash
wc -l agentic-apps-builder-template/.claude/skills/user-journey/SKILL.md
```

Expected: ~90 lines

**Step 3: Commit**

```bash
git add agentic-apps-builder-template/.claude/skills/user-journey/SKILL.md
git commit -m "feat: add user-journey skill to agentic app builder template"
```

---

### Task 5: Verify full structure and push

**Step 1: Verify complete directory structure**

```bash
find agentic-apps-builder-template/.claude -type f | sort
```

Expected:
```
agentic-apps-builder-template/.claude/CLAUDE.md
agentic-apps-builder-template/.claude/skills/user-journey/SKILL.md
agentic-apps-builder-template/.claude/skills/v1-scoping/SKILL.md
```

**Step 2: Verify git status is clean**

```bash
git status --porcelain
```

Expected: empty output (all committed)

**Step 3: Push branch and open PR**

```bash
git push -u origin HEAD
```

Then create PR:
```bash
gh pr create \
  --title "feat: add v1-scoping and user-journey skills to agentic app builder template" \
  --body "$(cat <<'EOF'
## Summary

- Adds `.claude/CLAUDE.md` to enforce the required workflow (v1-scoping → brainstorming → v1-scoping → user-journey → writing-plans)
- Adds `v1-scoping` skill with two phases: feasibility check before brainstorming, v1 filter after
- Adds `user-journey` skill that collaboratively walks through entry point, interactions, data flows, and edge cases

## Why

Without these guardrails, agents tend to over-scope v1s, dive into infeasible designs, and skip thinking through how users actually interact with the app. These skills force the right checkpoints at the right moments.

## Test plan

- [ ] Open a new Claude Code session in `agentic-apps-builder-template/`
- [ ] Ask the agent to build something (e.g., "build me a feedback form app")
- [ ] Verify the agent invokes `v1-scoping` before brainstorming
- [ ] Verify the agent invokes `v1-scoping` again after brainstorming with the 3-column table
- [ ] Verify the agent invokes `user-journey` with one section at a time
- [ ] Verify the agent ends by invoking `writing-plans`

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

| Task | Files Created | Commit |
|------|--------------|--------|
| 1 | Directory structure | — |
| 2 | `.claude/CLAUDE.md` | `feat: add CLAUDE.md workflow enforcement` |
| 3 | `.claude/skills/v1-scoping/SKILL.md` | `feat: add v1-scoping skill` |
| 4 | `.claude/skills/user-journey/SKILL.md` | `feat: add user-journey skill` |
| 5 | — | Push + PR |
