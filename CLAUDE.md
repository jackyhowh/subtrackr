# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

SubTrackr is a self-hosted subscription tracker: a single Go binary serving server-rendered HTMX pages backed by SQLite. There is no separate frontend build — templates and static assets are served directly.

## Commands

```bash
# Run the web server (defaults to :8080; reads ./data/subtrackr.db)
go run cmd/server/main.go        # or `go run .` from repo root (what Playwright uses)

# Build binaries
go build -o subtrackr cmd/server/main.go
go build -o subtrackr-mcp ./cmd/mcp     # MCP server (shares the same SQLite DB)

# Go tests (this is what CI runs)
go test ./...
go test -v ./internal/service/...                       # one package
go test -run TestParseReminderWindows ./internal/service # one test

# Playwright E2E tests — these auto-start the server via `go run .` on :8082
npm test                          # all browsers (chromium/firefox/webkit)
npx playwright test tests/i18n-coverage.spec.js --workers=1   # one spec
npm run test:headed               # watch it run

# Format (CI does not lint, but keep gofmt-clean)
go fmt ./...
```

Note the port split: the server defaults to **8080**, but `playwright.config.js` starts its own instance on **8082**. CI (`.github/workflows/test-build.yml`) runs `go build`, `go test ./...`, and a Docker build on every PR touching Go/template/web files.

## Architecture

Strict layered dependency flow, all wired by hand in [cmd/server/main.go](cmd/server/main.go):

```
handlers/  (Gin HTTP, request parsing, render templates or JSON)
   ↓
service/   (business logic — cost math, reminders, currency, notifications)
   ↓
repository/ (GORM data access, one repo per aggregate)
   ↓
models/    (GORM structs + domain methods like AnnualCost/MonthlyCost)
```

`main.go` constructs every repository, then service, then handler in order and injects dependencies via constructors (`NewXxx(...)`) — there is no DI framework. To add a feature you generally touch one file per layer plus a route registration in `setupRoutes`.

**Two route surfaces** (`setupRoutes` in main.go):
- Internal HTMX routes (`/`, `/subscriptions`, `/api/...`) behind session `AuthMiddleware` — these return HTML fragments, not JSON.
- Public REST API under `/api/v1/...` behind `APIKeyAuth` middleware — these return JSON. The same handler methods (`CreateSubscription`, etc.) serve both; they branch on content negotiation.

**Background schedulers**: two goroutines (`startRenewalReminderScheduler`, `startCancellationReminderScheduler`) launched from `main`. Each runs once ~30s after boot, then every 24h, wrapped in panic-recovery. They fan out to email, Pushover, and webhook channels; a send counts as "sent" if *any* channel succeeds, and the subscription's `LastReminder*` fields are persisted to avoid re-sending for the same renewal date.

**Database & migrations** ([internal/database/migrations.go](internal/database/migrations.go)): SQLite via GORM, file at `DATABASE_PATH`. `RunMigrations` AutoMigrates the simple models, then runs an **ordered slice of hand-written migration functions** (`migrateCurrencyFields`, `migrateReminderWindows`, …), then AutoMigrates `Subscription` last. When changing the schema, add a new migration function to that slice rather than relying on AutoMigrate alone — legacy databases have column-order quirks the join-table SQL works around.

**i18n** ([internal/i18n/](internal/i18n/), `web/locales/*.json`): flat dotted-key → string JSON dictionaries, one per language, loaded into a `Catalog` at startup. `en.json` is canonical; missing keys fall back to English (and a missing key falls back to itself, so a broken lookup is non-fatal). Templates call the `t`, `statusLabel`, and `scheduleLabel` funcmap helpers. Templates are re-read per request, so translation edits show on refresh, but adding a *language* requires a restart. See [web/locales/README.md](web/locales/README.md) and `tests/i18n-coverage.spec.js`.

**Templates**: server-rendered Go `html/template` in `templates/`, loaded individually (not via glob) in `loadTemplates` so a single broken template degrades gracefully — `dashboard.html`, `subscriptions.html`, and `error.html` are treated as critical and a parse failure there is fatal. Static assets are served from `web/static/`.

**Config & CLI**: `config.Load()` reads `PORT`, `DATABASE_PATH`, `GIN_MODE`, `FIXER_API_KEY` from env. `main` also handles offline admin commands before starting the server: `--reset-password` (`--new-password` for non-interactive), `--disable-auth`. Auth itself is optional — the app runs with no login until credentials are set up in Settings.

**Currency**: `service/currency.go` integrates Fixer.io (optional, key-gated). The free Fixer plan only allows EUR as base, so conversions are computed as cross-rates through EUR; rates are cached 24h in the `exchange_rates` table.

## Release Workflow

This project uses versioned branches for releases. Follow this workflow when working on new features or bug fixes.

### 1. Create a Versioned Branch

```bash
# Check current version
gh release list --limit 1

# Create and checkout versioned branch
git checkout -b v0.X.Y
```

### 2. Track Work with Beads

```bash
# Create beads issues for work items
bd create --title="Feature description (#GitHub-issue)" --type=feature --priority=2

# Update status when starting work
bd update <issue-id> --status=in_progress

# Close when complete
bd close <issue-id> --reason="Implemented in vX.Y.Z"
```

### 3. Create Draft Release Before Committing

```bash
# Create draft release with release notes
gh release create vX.Y.Z --draft --title "vX.Y.Z - Release Title" --notes "$(cat <<'EOF'
## What's New

### Feature Name (#issue)
- Description of changes

## Technical Changes
- List of technical changes
EOF
)"
```

### 4. Code Review

Before committing, run the code review agent:
- Check for code quality issues
- Verify security concerns
- Ensure best practices

### 5. Commit and Push

```bash
# Stage changes
git add <files>

# Commit with descriptive message
git commit -m "vX.Y.Z - Release Title

- Change 1
- Change 2"

# Push branch
git push -u origin vX.Y.Z
```

### 6. Create Pull Request

```bash
gh pr create --title "vX.Y.Z - Release Title" --body "$(cat <<'EOF'
## Summary
- Change summary

## Test Plan
- [ ] Test item 1
- [ ] Test item 2

Closes #issue1
Closes #issue2
EOF
)"
```

### 7. Comment on GitHub Issues

```bash
# Notify issue reporters
gh issue comment <issue-number> --body "Fixed in PR #XX. Description of fix."
```

### 8. Monitor CI and Merge

```bash
# Watch GitHub Actions
gh run watch <run-id> --exit-status

# Merge when CI passes
gh pr merge <pr-number> --merge --delete-branch

# Switch to main
git checkout main && git pull
```

### 9. Publish Release

```bash
# Publish the draft release
gh release edit vX.Y.Z --draft=false

# Verify
gh release view vX.Y.Z
```

## Beads Integration

This project uses beads for local issue tracking across sessions.

### Files
- `.beads/issues.jsonl` - Issue data (committed)
- `.beads/interactions.jsonl` - Audit log (committed)
- `.beads/beads.db` - Local cache (gitignored)

### Commands
- `bd ready` - Find available work
- `bd create` - Create new issue
- `bd update` - Update issue status
- `bd close` - Close completed issues
- `bd sync --from-main` - Sync from main branch

## Git Commit Guidelines

- Do not include AI attribution in commit messages
- Use conventional commit format
- Keep messages clear and descriptive
- Reference GitHub issue numbers where applicable
