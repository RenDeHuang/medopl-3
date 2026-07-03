# Development Worktree Policy

This repository uses short-lived Git worktrees for all medium or large changes.

## Worktree Rules

1. Do not develop medium or large changes directly on `main`.
2. Create one worktree per objective. Do not mix UI, contract, billing, auth, runtime, or documentation cleanup in the same worktree unless the change explicitly crosses those boundaries.
3. Keep product-design exploration worktrees short lived. At most one candidate design worktree may remain active after review; unselected design worktrees must be removed the same day.
4. Before merging a worktree back to `main`, run:
   - `npm test`
   - `npm run build`
   - `git diff --check`
5. After merging to `main`, push `main`, remove the physical worktree, delete the merged feature branch, and drop the task-specific stash.
6. Every closeout must check:
   - `git worktree list`
   - `git status --short --branch`
   - `git branch --merged main`
   - `git stash list`

## Repository Size Rules

1. Worktrees are disposable execution spaces, not archives.
2. Keep `node_modules` only where active development or verification needs it.
3. Do not commit generated build output from `dist/`.
4. Archive dated implementation records under `docs/history/**`; do not keep duplicate active process docs.
5. Delete stale local and remote branches after their work has landed or been intentionally abandoned.

## Contract And Test Rules

1. Active docs, contracts, and tests must represent current long-term truth.
2. Stage-specific tests are allowed only as temporary migration or cleanup guards with an owner and removal condition.
3. Do not keep tests that assert source line counts, raw source string wording, temporary UI copy, or old compatibility paths.
4. Prefer contract-driven tests over implementation-shape tests.
5. Move completed process evidence to `docs/history/**`.

