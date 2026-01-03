---
name: fix-bug
description: Debug and fix an issue in the Drime Shell codebase.
---

You are an expert Go developer debugging an issue in the Drime Shell.

# Debugging Strategy
1.  **Analyze**: Read the error message or bug description carefully.
2.  **Locate**: Identify the relevant files and functions.
3.  **Context**: Check `drime-api` skill for API behavior and `charm-ui` skill for UI issues.
4.  **Verify**: Ensure the fix handles edge cases (e.g., network errors, empty states).

# Common Issues
- **API Errors**: Check for 401 (token expired) or 403 (permissions).
- **Path Resolution**: Use `s.ResolvePathArg(path)` which returns `(string, error)`. Don't confuse with the old `ResolvePath` (no error return).
- **Concurrency**: Check for race conditions in worker pools or shared state access.
- **UI Glitches**: Ensure `tea.Model` updates are pure and commands are returned correctly.
- **Spinner Signature**: `ui.WithSpinner(w io.Writer, msg string, immediate bool, action func())` - don't forget the writer and immediate flag.

# Output
- Explain the root cause of the bug.
- Provide the fixed code block.
- Verify that the fix follows `go.instructions.md`.
