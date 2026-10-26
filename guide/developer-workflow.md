# Developer Workflow

This document is the practical "how do I safely work in this repo?" guide.

It is written for someone who may be inheriting the project with incomplete context, so it includes both setup and safety habits.

## 1. First practical goal

Your first goal is not "understand every file".

Your first goal is:

- get the app running
- click through the main flows
- learn how one action travels through the stack

That gives you a stable mental model before you attempt any refactor.

## 2. Toolchain

The repository currently expects:

- Go, as declared in `go.mod`
- Node.js
- npm
- Wails CLI

### Current Go declaration

`go.mod` currently declares:

```text
go 1.25.0
```

### Install Wails CLI if needed

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 3. Common commands

Run these from the repository root unless noted otherwise.

### Start local development

```bash
wails dev
```

This is the most important command because it:

- runs the Go desktop process
- serves and watches the frontend
- regenerates Wails contract files as needed

### Build production artifacts

```bash
wails build
```

### Run backend tests

```bash
go test ./backend/...
```

### Install frontend dependencies manually

```bash
cd frontend
npm install
```

### Build the frontend directly

```bash
cd frontend
npm run build
```

## 4. Environment behavior

The app uses `.env` plus runtime environment detection.

Important flag:

- `DEVELOPMENT`

This flag influences:

- app title
- single-instance ID
- database filename
- screenshot storage path
- FES API base URL choice
- some development-vs-production timing behavior

### Security note

Treat `.env` as sensitive.

Do not:

- paste real secrets into docs
- paste them into tickets
- commit replacements carelessly
- assume every existing secret should be shared widely

## 5. What to do on day 1

1. run `npm install` in `frontend` if needed
2. run `wails dev`
3. confirm the window opens
4. try login or offline login depending on what test credentials exist
5. visit:
   - Home
   - one Task page
   - Activity Logs
   - Account Settings
6. read:
   - `main.go`
   - `backend/app/app.go`
   - `frontend/src/App.tsx`
   - `frontend/src/store/AuthContext.ts`
7. trace one full flow end-to-end

## 6. How to trace a change safely

### If you are changing frontend-only presentation

Read in this order:

1. the page or component
2. any shared UI primitives it uses
3. store fields it reads
4. event subscriptions it depends on

Still verify whether backend assumptions are baked into the UI.

### If you are changing backend business logic

Read in this order:

1. the `App` method exposed to the frontend
2. the service method it delegates to
3. the repository methods below it
4. related runtime events
5. the data model it touches

### If you are changing sync behavior

Read before editing:

1. `backend/service_classes/sync.service.go`
2. `guide/data-and-sync.md`
3. the related task or attendance lifecycle methods
4. the frontend screens that display sync state

### If you are changing schema

Read before editing:

1. `backend/db/models/*.go`
2. `backend/db/migrations.go`
3. `backend/db/schema_tables.go`
4. `backend/db/indexes.go`
5. migration tests

## 7. Recommended change recipes

### Add a new backend method for the frontend

1. add or update a method on `backend/app.App`
2. keep the facade method small
3. add deeper logic in a service
4. use repositories for DB access
5. run the Wails dev/build workflow so bindings regenerate
6. update the frontend import and usage
7. test the full flow

### Add or change database fields

1. update the model
2. update migration and repair logic
3. update repositories
4. update services
5. update exported shapes if the frontend needs the field
6. add or update tests
7. verify older local DBs still have a safe path forward

### Change task-tracking behavior

1. read `Task.tsx`
2. read `App.StartTracking` and `App.StopTracking`
3. read `TaskService`
4. read `SyncService`
5. test both online and offline thinking, even if you only change one path

## 8. Good debugging workflow

When something breaks:

1. reproduce it in `wails dev`
2. note the exact screen and action
3. check whether the frontend called the expected Wails method
4. check the backend logs
5. inspect the relevant local DB rows
6. check whether runtime events were emitted
7. confirm whether the issue is local-state, backend-state, or remote-sync related

### Best-first questions

- Did the local record get created?
- Did the sync status change?
- Was the app online according to the backend?
- Did attendance have the remote ID the task needed?
- Did the frontend miss the event that would refresh the UI?

## 9. Current mismatches and caution areas

### Production versions come from the release tag

The production workflow stamps the `vMAJOR.MINOR.PATCH` tag into the build
identity, legacy backend version, and Windows installer metadata. Local builds
use the defaults in the source files. Follow
[`stable-release-workflow.md`](./stable-release-workflow.md) when publishing.

### Auth has more than one visible source

The frontend still has Firebase auth listener code, but the practical session flow depends heavily on Go-side FES API login and local state hydration.

### Screenshot behavior spans many layers

Do not change screenshot UX without checking:

- frontend release flags
- screenshot capture service
- sync behavior
- storage path helpers
- Backblaze upload logic

## 10. What files you usually should not hand-edit

### Generated files

- `frontend/wailsjs/**`

These should be regenerated, not manually modified.

### Build outputs

- artifacts under build output folders

Rebuild them instead of editing them.

### Runtime-generated data

- local databases
- screenshot files
- log files

Inspect them for debugging, but do not treat them like source files.

## 11. Release checklist

For the current production publishing process, follow
[`stable-release-workflow.md`](./stable-release-workflow.md). Code is merged into
`main` in `FES-Paul/wails-app`; a version tag triggers publication of the installer,
checksum, and manifest to `FES-Paul/FES-APP-TRACKER`. Stable apps check that public
release repository. Merging alone does not publish an update.

Before a release, verify all of these:

1. the release tag is an unused version greater than the current production release
2. `wails.json` metadata is correct
3. screenshot-related flags and constants match intended release behavior
4. update-check behavior points to the right GitHub release source
5. no secrets are being newly exposed
6. backend tests pass
7. manual smoke test covers:
   - login
   - task start
   - task stop
   - logout
   - activity logs
   - manual resync

## 12. Suggested smoke-test workflow

Use this after risky changes:

1. start the app
2. log in online
3. confirm tasks load
4. start a non-Break task
5. wait long enough for at least one screenshot in dev mode if screenshot work changed
6. stop the task
7. confirm sync state updates
8. open activity logs
9. confirm the task record and attendance appear
10. log out

## 13. Safer versus riskier refactors

### Usually safer

- documentation
- UI text and layout cleanup
- local helper extraction
- improving backend tests
- small repository query refactors with good test coverage

### Usually riskier

- sync queue logic
- migrations
- attendance/task ordering rules
- pointer wiring in `App`
- screenshot path and upload behavior
- auth/session hydration

## 14. Useful reading order by problem type

### Login problems

1. `frontend/src/pages/Auth/Login.tsx`
2. `backend/app/app.go`
3. `backend/service_classes/auth.service.go`
4. `backend/service_classes/fes_client.service.go`
5. `backend/service_classes/user.service.go`

### Tracking problems

1. `frontend/src/pages/Main/Task.tsx`
2. `backend/app/app.go`
3. `backend/service_classes/task.service.go`
4. `backend/service_classes/sync.service.go`
5. `backend/repository/task.repo.go`

### Activity log or summary problems

1. `frontend/src/pages/Main/ActivityLogs.tsx`
2. `backend/services/activity_log.go`
3. `backend/services/summary.go`
4. `guide/data-and-sync.md`

### Startup problems

1. `main.go`
2. `backend/app/app.go`
3. `backend/db/db.go`
4. `wails.json`

## 15. When to slow down and ask bigger questions

Pause and re-evaluate before coding if your change affects:

- database shape
- remote API contracts
- sync ordering
- auth ownership
- screenshot retention or privacy expectations
- release/update behavior

Those areas have consequences beyond one file.

## Read next

- [`guide/takeover-playbook.md`](./takeover-playbook.md) for a practical first-week plan
- [`guide/troubleshooting.md`](./troubleshooting.md) for incident debugging
