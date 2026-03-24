---
name: writing-plans
description: Use when you have a spec or requirements for a multi-step task, before touching code
---

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI.

Assume they are a nontechnical person, know almost nothing about our toolset or problem domain. Assume they don't know good test design very well.

**Announce at start:** "I'm using the writing-plans skill to create the implementation plan."

**Context:** This should be run in a dedicated worktree (created by brainstorming skill).

**Save plans to:** `docs/plans/YYYY-MM-DD-<feature-name>.md`

## Bite-Sized Task Granularity

**Each step is one action (2-5 minutes):**
- "Write the failing test" - step
- "Run it to make sure it fails" - step
- "Implement the minimal code to make the test pass" - step
- "Run the tests and make sure they pass" - step
- "Commit" - step

## Plan Document Header

**Every plan MUST start with this header:**

```markdown
# [Feature Name] Implementation Plan

**Goal:** [One sentence describing what this builds]

**Architecture:** [2-3 sentences about approach]

---
```

## Task Structure

````markdown
### Task N: [Component Name]

**Task description**
2-5 sentence description of the task and key outcomes.

**Files:**
- Create: `exact/path/to/file.py`
- Modify: `exact/path/to/existing.py:123-145`
- Test: `tests/exact/path/to/test.py`

**Step 1: Write the failing test**

```python
def test_specific_behavior():
    result = function(input)
    assert result == expected
```

**Step 2: Run test to verify it fails**

Run: `pytest tests/path/test.py::test_name -v`
Expected: FAIL with "function not defined"

**Step 3: Write minimal implementation**

```python
def function(input):
    return expected
```

**Step 4: Run test to verify it passes**

Run: `pytest tests/path/test.py::test_name -v`
Expected: PASS

**Step 5: Run `make build` to verify your code builds**

Run: `make build`
Expected: no errors

## Remember
- Exact file paths always
- Complete code in plan (not "add validation")
- Exact commands with expected output
- Reference relevant skills with @ syntax
- DRY, YAGNI, TDD, frequent commits

## Final Task (always include as the last task in every plan)

Every plan MUST end with this task:

### Task N: Walk the user through the app

**Task description**
Implementation is complete. Show the user what was built and how to use it.

**Steps:**
1. Summarize what was built in 2–3 plain-English sentences (no code, no jargon)
2. Walk through the exact steps a user takes to try the feature — what to click, what to type, what they'll see
3. Point out any setup required (e.g. connecting an integration, entering a URL, granting a permission)
4. Tell the user what to do if something looks wrong

---

## Execution Handoff

After saving the plan, immediately execute:

**"Plan complete and saved to `docs/plans/<filename>.md`.**

**If plan is an existing feature**
- Edit the relevant code

**If plan is a new feature**
- Be sure to use the adding-features skill

