# Design: v1-scoping and user-journey Skills for Agentic App Builder Template

**Date:** 2026-03-06
**Author:** brandon.man@applied.co

## Overview

Add two new Claude Code skills to the `agentic-apps-builder-template` that enforce better scoping discipline and surface user journey clarity before any implementation begins. These skills reduce wasted agent effort on infeasible requests and ensure the smallest deployable product is always the target.

## Problem

Without structured guidance, agents tend to:
- Dive into brainstorming features that can't actually be built (bad external dependencies, too complex, wrong platform capabilities)
- Over-scope v1 implementations, generating large amounts of code that's hard to test and iterate on
- Skip thinking through how a real user interacts with the app, leading to design gaps discovered late

## Solution

Two new skills in `agentic-apps-builder-template/.claude/skills/` with a `CLAUDE.md` that enforces the workflow chain.

---

## Directory Structure

```
agentic-apps-builder-template/
└── .claude/
    ├── CLAUDE.md                  # Required workflow for this template
    └── skills/
        ├── v1-scoping/
        │   └── SKILL.md
        └── user-journey/
            └── SKILL.md
```

---

## Skill 1: v1-scoping

### Discovery

**Name:** `v1-scoping`
**Description:** Use when starting a new feature or app build — before brainstorming to assess feasibility, and after brainstorming to cut the design to the smallest deployable v1.

### Two Phases

#### Phase 1: Feasibility (before brainstorming)

Ask three targeted questions, one at a time:
1. What does success look like? (one sentence — anchors scope)
2. What external dependencies does this require? (APIs, auth, real-time data, third-party services)
3. What's the hardest part? (surfaces hidden complexity)

Based on answers, either:
- Confirm feasibility and propose a rough v1 boundary (e.g., "v1 = static data, no auth, no real-time")
- Flag infeasible parts and propose what can be built instead

**REQUIRED NEXT:** invoke `brainstorming`

#### Phase 2: Detailed scoping (after brainstorming)

Take the full brainstormed design and apply the v1 filter. Output a 3-column table:

| In v1 | Deferred to v2 | Cut entirely |
|-------|---------------|--------------|

Guiding questions for each feature in the design:
- Can this be hardcoded or faked in v1?
- Does v1 work without this?
- Does cutting this still leave something deployable and testable?

**REQUIRED NEXT:** invoke `user-journey`

### Standalone Use

Can be invoked mid-implementation when scope creep occurs. In this case, run only Phase 2.

---

## Skill 2: user-journey

### Discovery

**Name:** `user-journey`
**Description:** Use after v1 is scoped to walk through how a real user interacts with the app — what they click, what the app shows, and how data flows at a high level.

### Structure (collaborative, section by section)

Agent drafts each section, presents it, waits for user correction, then moves on.

#### Section 1: Entry point
- How does the user get to the app?
- What do they see first? (landing page, login screen, empty state)

#### Section 2: Core interaction loop
For each of the 2-4 key user actions:
- What they see
- What they click / type / submit
- What the app does in response (high level: "backend fetches X from Y, returns Z")

#### Section 3: Data flows
For each user action, what data is pulled/pushed?
- Where does it come from?
- Example: "user submits form → POST /api/submit → writes to Postgres → returns confirmation"

#### Section 4: Edge cases
- What happens when something goes wrong?
- Empty states, error messages, loading states

#### Closing summary
Agent writes a one-paragraph plain-English summary of the full journey, suitable for a non-technical reviewer.

**REQUIRED NEXT:** invoke `writing-plans`

---

## Integration: CLAUDE.md

A `CLAUDE.md` at `agentic-apps-builder-template/.claude/CLAUDE.md` enforces the required workflow for any new feature or app built with this template:

```
Required workflow for new features/apps:
1. Invoke v1-scoping (phase 1: feasibility check)
2. Invoke brainstorming
3. Invoke v1-scoping (phase 2: cut to smallest deployable v1)
4. Invoke user-journey
5. Invoke writing-plans
```

Each skill also cross-references its neighbours so the chain is self-reinforcing even if CLAUDE.md is not read.

---

## Full Flow

```
v1-scoping (phase 1)
    ↓  feasibility confirmed, rough v1 boundary set
brainstorming
    ↓  full design produced
v1-scoping (phase 2)
    ↓  In v1 / Deferred / Cut table produced
user-journey
    ↓  entry point, interactions, data flows, edge cases approved
writing-plans
    ↓  implementation plan ready
```

---

## What This Is Not

- These skills do not replace brainstorming — they wrap it
- These skills do not generate code — they ensure what gets built is the right thing
- v1-scoping phase 2 does not redesign — it prunes the brainstormed design
