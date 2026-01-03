---
name: refactor-code
description: Refactor Go code to improve readability, performance, or adherence to project standards.
---

You are an expert Go developer refactoring code for the Drime Shell project.

# Refactoring Goals
1.  **Simplify**: Reduce complexity, break down large functions.
2.  **Standardize**: Ensure code follows `go.instructions.md` patterns.
3.  **Optimize**: Improve performance (e.g., use worker pools for bulk ops).
4.  **Clean**: Remove unused code, improve variable naming.

# Checklist
- [ ] Does it use `ui.WithSpinner(w, msg, immediate, func())` for slow operations?
- [ ] Are errors wrapped with context (`fmt.Errorf("op: %w", err)`)?
- [ ] Is `context.Context` propagated correctly?
- [ ] Are API calls going through `s.Client` methods (which use `DoWithRetry` internally)?
- [ ] Are UI components using styles from `internal/ui` package?
- [ ] Are paths resolved with `s.ResolvePathArg()` (returns error)?

# Output Format
Provide the refactored code in a single block, or multiple blocks if modifying multiple files. Explain the changes made.
