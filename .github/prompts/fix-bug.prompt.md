---
name: fix-bug
description: Debug and fix an issue in the Drime Shell codebase using systematic root cause analysis.
---

You are an expert Go developer debugging an issue in the Drime Shell.

# The Law

**ALWAYS find root cause before attempting fixes.**

If ≥3 fix attempts have failed → STOP and question the architecture or your understanding.

# Phase 1: Root Cause Investigation

1. **Reproduce**: Create a reliable way to trigger the bug
2. **Instrument**: Add debug logging to trace execution
3. **Trace backwards**: Start from symptom, work back to source
4. **STOP when found**: Don't fix until you understand the actual cause

## Investigation Questions
- What exact error/behavior is observed?
- What is the expected behavior?
- When did this start happening?
- What changed recently?

# Phase 2: Hypothesis Testing

Before any fix:
1. **State hypothesis**: "I believe X causes Y because Z"
2. **Predict outcome**: "If I change A, then B should happen"
3. **Test prediction**: Make minimal change, verify prediction
4. **If wrong**: Return to Phase 1, don't stack guesses

# Phase 3: Defense in Depth

After finding root cause, add validation at multiple layers:

| Layer | Purpose | Example |
|-------|---------|---------|
| **Entry Point** | Validate inputs immediately | Check args in command func |
| **Business Logic** | Re-validate at critical points | Verify cache entry exists |
| **Environment Guards** | Check invariants | Confirm session not nil |
| **Debug Instrumentation** | Temporary logging | Log state at key points |

# Phase 4: Verify Fix

- Run the reproduction steps → bug should be gone
- Run `go test -race ./...` → no regressions
- Remove debug instrumentation
- Document what was wrong and why

# Common Drime Issues

| Issue | Likely Cause | Check |
|-------|--------------|-------|
| **API 401/403** | Token expired or wrong workspace | `s.Client` auth, `s.WorkspaceID` |
| **Path not found** | Cache stale or wrong resolution | `s.ResolvePathArg()` returns error |
| **Race condition** | Shared state in worker pool | Run with `-race` flag |
| **UI glitch** | `tea.Model` not returning command | Check `Update()` returns `tea.Cmd` |
| **Spinner issues** | Wrong signature | `ui.WithSpinner(w, msg, immediate, fn)` |

# Output

1. **Root cause**: What was actually wrong
2. **Fix**: The code change (follows `go.instructions.md`)
3. **Verification**: How you confirmed it works
4. **Prevention**: Any defensive checks added
