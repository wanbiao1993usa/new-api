# Ergouzi Branch Workflow

> Updated: 2026-05-26

This document is for Ergouzi teammates working directly in this `new-api` repository.

## What Changed

`main` is now the Ergouzi customized main branch.

The previous `main` was an older New API / fork branch and had stopped representing the active Ergouzi development line. The historical customized branch `codex/subscription-redemption-codes` has been fast-forwarded into `main`.

Current branch roles:

| Branch | Purpose |
|---|---|
| `main` | Ergouzi customized main branch. Start normal development here. |
| `upstream-sync` | Tracks upstream New API `main`. Do not put Ergouzi business changes here. |
| `codex/subscription-redemption-codes` | Historical customized branch. It currently points to the same commit as `main`, but should not be used for new work. |
| `feature/*` / `fix/*` | Normal feature and fix branches, created from `main`. |
| `sync/upstream-YYYYMMDD` | Temporary branch for integrating upstream New API changes into Ergouzi `main`. |

Current reference points:

| Ref | Commit |
|---|---|
| `origin/main` | `c1c0356a3abe702da498d5d2472ccb27a0e45027` |
| `origin/codex/subscription-redemption-codes` | `c1c0356a3abe702da498d5d2472ccb27a0e45027` |
| `origin/upstream-sync` | `65f8afe92276203a33413a1bd5d5172ccd46e04e` |

## Sync Your Local Repository

If you have no local uncommitted changes:

```bash
git fetch origin --prune
git checkout main
git pull --ff-only origin main
```

Fetch the upstream baseline branch:

```bash
git fetch origin upstream-sync:upstream-sync
git branch --set-upstream-to=origin/upstream-sync upstream-sync
```

Start new work from `main`:

```bash
git checkout main
git pull --ff-only origin main
git checkout -b feature/your-feature
```

## If You Are Still On `codex/subscription-redemption-codes`

Check your local status first:

```bash
git status --short --branch
```

If the working tree is clean:

```bash
git fetch origin --prune
git checkout main
git pull --ff-only origin main
```

If your local `main` does not exist or is not trustworthy:

```bash
git fetch origin --prune
git checkout -B main origin/main
```

## If You Have Local Uncommitted Changes

Do not switch branches blindly. Stash first:

```bash
git status --short --branch
git stash push -u -m "before-main-sync"
git fetch origin --prune
git checkout main
git pull --ff-only origin main
git stash pop
```

If `stash pop` reports conflicts, resolve them before continuing. Do not commit unresolved conflict markers.

## Upstream Sync Rules

Most developers do not need to configure the official upstream remote. Use `origin/upstream-sync` as the shared upstream baseline.

The person doing upstream sync may configure:

```bash
git remote add upstream https://github.com/QuantumNous/new-api.git
git remote set-url --push upstream DISABLED
git fetch upstream main
```

Rules:

- Do not write Ergouzi business code on `upstream-sync`.
- Do not push to the official upstream repository.
- Keep `upstream-sync` as a clean upstream tracking branch.
- For upstream integration, create a temporary branch from Ergouzi `main`:

```bash
git checkout main
git pull --ff-only origin main
git checkout -b sync/upstream-YYYYMMDD
git fetch upstream main
git merge upstream/main
```

The sync pull request should record:

- upstream from / to commits;
- Ergouzi `main` base commit;
- conflict files and resolution notes;
- database or config changes;
- verification results;
- deploy recommendation.

## Do Not

- Do not start new work from `codex/subscription-redemption-codes`.
- Do not commit Ergouzi changes to `upstream-sync`.
- Do not force-push `main`.
- Do not merge upstream changes directly into `main` without a sync branch and verification.
- Do not push to `upstream`; keep its push URL disabled.

## When In Doubt

Stop and share:

```bash
git status --short --branch
git branch -vv
git remote -v
```

Then ask before running `reset`, `rebase`, `push --force`, or deleting branches.
