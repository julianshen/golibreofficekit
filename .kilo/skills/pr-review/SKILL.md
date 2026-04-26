---
name: pr-review
description: Comprehensive pull request review that dispatches specialized sub-reviews (code quality, tests, error handling, comments, type design, simplification) and aggregates the findings into a prioritized action plan. Use this whenever the user wants to review a PR, review their recent changes before committing, prepare to open a PR, audit test coverage, check error handling for silent failures, verify comment accuracy, evaluate new type designs, or simplify code after review — even when they just say "review my changes", "check this before I push", or "am I ready to open a PR".
---

# PR Review

Run a focused, multi-aspect review of pending changes and turn the results into a short action plan. Adapted from the [pr-review-toolkit plugin](https://github.com/anthropics/claude-code/tree/main/plugins/pr-review-toolkit).

## Phase 1 — Scope the review

Figure out what actually changed before launching anything:

1. Run `git status` and `git diff --name-only` (plus `git diff main...HEAD` if on a feature branch) to list changed files.
2. If the user mentioned a PR number or branch, pull it with `gh pr view <n>` / `gh pr diff <n>`.
3. Skim the diff. Classify each file by what kinds of review it needs — this drives which specialists you launch in Phase 2.

If the diff is empty, fall back to the most recently modified files the user pointed at or that you edited earlier in this conversation. Don't invent scope.

## Phase 2 — Pick the right specialists

Six specialist reviews are available. Match them to the diff — don't run all six reflexively. Each one is a subagent; invoke via the Agent tool with the `subagent_type` shown.

| Aspect | When it applies | `subagent_type` |
|---|---|---|
| General code quality, bugs, convention violations | Any code change | `pr-review-toolkit:code-reviewer` |
| Test coverage gaps, weak assertions, missing edge cases | Added/changed behavior, new tests, or suspiciously no tests | `pr-review-toolkit:pr-test-analyzer` |
| Silent failures, swallowed errors, bad fallbacks | `try/catch`, `.catch()`, `err != nil`, fallback branches, recover/rescue | `pr-review-toolkit:silent-failure-hunter` |
| Comment accuracy and rot | Added or modified comments/docstrings | `pr-review-toolkit:comment-analyzer` |
| Type design (encapsulation, invariants) | New or reworked types, schemas, interfaces, data models | `pr-review-toolkit:type-design-analyzer` |
| Simplification after review passes | Code works but feels dense; use last, not first | `pr-review-toolkit:code-simplifier` |

Rules of thumb:
- **Always** include `code-reviewer` unless the user scoped you to something narrower.
- Include `silent-failure-hunter` whenever error-handling code moved, even slightly — this is the highest-leverage catch.
- Skip `type-design-analyzer` on pure refactors where no types changed; it produces noise.
- Hold `code-simplifier` back until the other reviews have been addressed. Simplifying code that has latent bugs bakes the bugs in.

If the user passed an explicit list ("just tests and errors"), honor that and skip the rest.

## Phase 3 — Launch reviews in parallel

Dispatch the chosen specialists in a **single message with multiple Agent tool calls** so they run concurrently. Brief each one with:

- The exact files or diff range to review (not "the whole repo").
- Any relevant project context — CLAUDE.md conventions, the PR's stated goal, language/framework.
- What form the reply should take: prioritized findings with `file:line` references, and nothing else.

Each specialist already knows how to do its job — don't re-describe the methodology to it. Just hand over scope + context.

## Phase 4 — Aggregate into an action plan

Wait for every specialist to return. Then consolidate — don't just stack their reports. Users drown in six independent reports; they act on one prioritized list.

Produce this structure:

```markdown
# PR Review Summary

## Critical (must fix before merge)
- [specialist] Short issue — `path/to/file.ts:42`
  Why it matters in one line.

## Important (should fix)
- [specialist] …

## Suggestions (nice to have)
- [specialist] …

## Strengths
- Brief, honest. Skip if there's nothing genuine to say.

## Recommended next step
One concrete action — usually "fix the critical items, then re-run `silent-failure-hunter` on the changed files."
```

Deduplicate findings that multiple specialists raised — attribute once, don't repeat. Drop low-confidence nitpicks; a long list of minor style points buries the signal.

## Phase 5 — Offer to fix or iterate

After presenting the summary, ask the user what they want to do: fix the critical items now, re-run a specific specialist after fixes, or just note the findings and move on. Don't start editing unprompted — PR review is advisory by default.

## Common invocation patterns

- **"Review my changes"** → Phase 1 from `git diff`, pick specialists from what's in the diff, run in parallel, summarize.
- **"Am I ready to open a PR?"** → Full review including `pr-test-analyzer` and `comment-analyzer`. End with an explicit ready/not-ready call.
- **"Check for silent failures in the new API client"** → Scope narrowly; run only `silent-failure-hunter`.
- **"Simplify this after the review passes"** → Only `code-simplifier`. Confirm other reviews are green first; if not, warn and ask before proceeding.

## Why this shape

The point of splitting the review into specialists is that each one has a narrow lens and catches things a generalist sweep misses — a single "review this PR" prompt reliably under-weights test coverage, silently-swallowed errors, and type design. Running them in parallel keeps wall-clock time low; aggregating keeps cognitive load low for the user. Holding `code-simplifier` until the end prevents the common failure mode where cosmetic cleanups mask real bugs.
