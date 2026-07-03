# Developer Workflow

This document is the practical operating guide for working on the project.

## 1. Recommended first-day checklist

If you are new to the codebase:

1. Read `guide/README.md`
2. Read `guide/architecture.md`
3. Run the app locally with `wails dev`
4. Log in and click through:
   - login
   - home
   - one task page
   - activity logs
   - account settings
5. Read the backend or frontend deep dive that matches the code you need to change

## 2. Toolchain

The repository currently expects:

- Go as declared in `go.mod` (`go 1.25.0`)
- Node.js for the frontend toolchain
- npm
- Wails CLI

Install Wails CLI if needed:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 3. Common commands

From the repository root:

### Start local development

```bash
wails dev
```

This gives you:

- the Go desktop process
- the frontend dev server/watch flow
- generated Wails bindings in the frontend

### Build the app

```bash
wails build
```

### Run backend tests

```bash
go test ./backend/...
```

### Install frontend dependencies manually if needed

```bash
cd frontend
npm install
```

## 4. Environment behavior

The current repo contains a `.env` file and the runtime checks `DEVELOPMENT`.

Important notes:

- the app uses `DEVELOPMENT`, not a more conventional `GO_ENV`, to switch certain behaviors
- dev/prod differences affect:
  - database filename
  - screenshot storage path
  - single-instance app id suffix
  - some timing behavior

Security note:

- `.env` currently contains sensitive-looking integration values
- treat it as secret material
- do not copy real values into documentation, issues, or public commits

## 5. How to trace a change safely

### If you are changing frontend UI only

Start with:

- the page component
- any related custom components
- the Zustand store fields it reads or writes
- the Wails binding it calls

Then confirm whether backend changes are actually required.

### If you are changing backend business logic

Start with:

- the exported `App` method
- the service class it delegates to
- the repository methods it uses
- any runtime events emitted back to the frontend

### If you are changing sync behavior

Read first:

- `backend/service_classes/sync.service.go`
- `guide/data-and-sync.md`

Then inspect:

- related UI in `ActivityLogs.tsx`
- any task or attendance lifecycle methods that create local records

### If you are changing schema or local data behavior

Read first:

- `backend/db/migrations.go`
- `backend/db/schema_tables.go`
- related model files under `backend/db/models`

Avoid adding schema changes without understanding the canonical repair approach already used in this repo.

## 6. How to add a new backend method for the frontend

Use this pattern:

1. add the method on `backend/app.App`
2. keep the method small and facade-oriented
3. move real business logic into a service if needed
4. use existing repository/service patterns where possible
5. run `wails dev` or the relevant Wails workflow so bindings regenerate
6. update the frontend to call the generated binding

Checklist:

- does it need auth guards?
- does it need stable user-facing errors?
- does it need runtime event emission?
- does it change any Wails-exported model shape?

## 7. How to add or change database fields

Use this pattern:

1. update the model in `backend/db/models`
2. update canonical migration/repair logic
3. update any related indexes if query patterns need them
4. update repositories and services
5. update generated frontend assumptions if exported shape changes
6. add or update migration tests

Do not rely on `AutoMigrate` alone. This codebase intentionally uses a more controlled migration strategy.

## 8. How to debug sync issues

Practical workflow:

1. reproduce the issue with `wails dev`
2. inspect logs
3. inspect SQLite record state
4. check whether the frontend received the expected runtime event
5. check whether the record is `failed` or `blocked`
6. confirm whether attendance dependency is satisfied before task sync

When investigating manually, the most useful fields are:

- `sync_status`
- `last_error`
- `attempt_count`
- `last_attempt_at`
- `login_logout_id`
- `productivity`

## 9. Current feature flags and mismatches to watch

### Screenshots

Current files disagree:

- frontend release flag says enabled
- backend screenshot constant says disabled
- backend Backblaze constant says disabled

Before working on screenshot UX or upload behavior, decide which product mode is actually intended.

### Version numbers

Current version values are not aligned across:

- `backend/app/app.go`
- `wails.json`
- old README content
- footer fallback value

Before a release, update all version declarations in one pass.

### Authentication assumptions

Do not assume Firebase is the single source of auth truth. The operational login path is driven by the FES API through Wails.

## 10. Release checklist

Before cutting a release, verify:

1. version values are aligned
2. `wails.json` metadata is correct
3. screenshot flags match the intended release behavior
4. update-check behavior points at the correct GitHub release stream
5. no sensitive secrets are being newly exposed
6. `go test ./backend/...` passes
7. manual smoke test covers:
   - login
   - task start/stop
   - logout
   - activity logs
   - manual resync

## 11. Files you normally should not edit directly

- `frontend/wailsjs/**`
  These are generated bindings.

- build outputs under `build/`
  Rebuild them instead of editing.

- local databases and runtime-generated screenshot files
  Use them for debugging, not source edits.

## 12. Good first refactors, and dangerous refactors

### Safer cleanup targets

- README and documentation
- purely presentational frontend cleanup
- local helper extraction inside a page or service
- additional backend tests

### Higher-risk areas

- sync queue and retry logic
- migration repair logic
- attendance/task ordering assumptions
- cross-service pointer wiring on `App`
- feature-flag behavior for screenshots and uploads

## 13. Recommended code-reading order by task type

### For auth problems

1. `frontend/src/pages/Auth/Login.tsx`
2. `backend/app/app.go`
3. `backend/service_classes/auth.service.go`
4. `backend/service_classes/fes_client.service.go`
5. `backend/service_classes/user.service.go`

### For tracking problems

1. `frontend/src/pages/Main/Task.tsx`
2. `backend/app/app.go`
3. `backend/service_classes/task.service.go`
4. `backend/repository/task.repo.go`
5. `backend/service_classes/sync.service.go`

### For activity log problems

1. `frontend/src/pages/Main/ActivityLogs.tsx`
2. `backend/services/activity_log.go`
3. `backend/services/summary.go`
4. `guide/data-and-sync.md`

### For startup or packaging problems

1. `main.go`
2. `wails.json`
3. `backend/app/app.go`
4. `backend/db/db.go`

## 14. Testing guidance

The backend already has tests concentrated around risky areas. Extend those patterns instead of creating isolated one-off tests in unrelated folders.

Especially important test areas:

- migrations
- sync guards and state transitions
- task lifecycle edge cases
- screenshot record hydration behavior
- app guard behavior

The frontend currently relies heavily on manual testing, so describe manual verification clearly in pull requests or change notes.

## 15. Read next

- `guide/backend.md`
- `guide/frontend.md`
- `guide/data-and-sync.md`
