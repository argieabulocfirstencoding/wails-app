# FES App Tracker Takeover Guide

This folder is the handover guide for the `wails-app` codebase.

It is written for a developer who may be:

- new to Go
- new to Wails
- new to desktop apps
- new to SQLite
- new to this company-specific workflow
- taking over the project with limited context

This guide is intentionally verbose. It explains not only what the code does, but also why the code is shaped this way and where a new maintainer should be careful.

## What this application is

FES App Tracker is a desktop time-tracking application.

At a practical level, the app lets a user:

- log in to the company's remote FES system
- create or resume a local attendance session
- start and stop work on tasks
- save task notes and activity descriptions
- capture screenshots while a task is running
- store work data locally when offline
- sync that local data to the remote FES API when online again

This means the app is not "just a frontend" and not "just an API client". It is a stateful desktop program with:

- a local database
- background synchronization
- OS integration
- file-system storage
- cloud upload support

## What makes this project unusual

Many web developers expect a simple pattern:

`frontend -> HTTP API -> database`

This project is different.

The main runtime pattern is:

```text
React UI
  -> generated Wails bindings
    -> Go App facade
      -> service classes
        -> repositories
          -> SQLite

Go services
  -> remote FES API
  -> local file system
  -> OS keyring
  -> Backblaze B2
```

Important consequence:

- the frontend does not directly call the FES API for core product behavior
- the frontend calls Go methods exposed by Wails
- the Go backend owns most business rules
- the SQLite database is not a throwaway cache

## If you only remember 10 things

If you are in a hurry, these are the most important truths about the app:

1. The local SQLite database is the app's operational memory. Protect it.
2. Offline behavior is a first-class requirement, not an edge case.
3. The React frontend talks to Go through Wails bindings, not normal REST routes.
4. `backend/app/app.go` is the Wails-facing facade. It should stay thin when possible.
5. `backend/service_classes/` is the main orchestration layer, but `backend/services/` still contains important active code.
6. Task syncing depends on attendance syncing first.
7. Screenshot capture, screenshot metadata, and screenshot upload are spread across multiple layers.
8. There are several sources of version truth right now, and they are not fully aligned.
9. Wails-generated files are contract artifacts. Do not hand-edit them.
10. When in doubt, trace one complete user action from page -> binding -> `App` method -> service -> repository -> database.

## Recommended reading order

If you are completely new, read the docs in this order:

1. [`guide/README.md`](./README.md)
2. [`guide/takeover-playbook.md`](./takeover-playbook.md)
3. [`guide/architecture.md`](./architecture.md)
4. [`guide/glossary.md`](./glossary.md)
5. [`guide/developer-workflow.md`](./developer-workflow.md)
6. [`guide/backend.md`](./backend.md) or [`guide/frontend.md`](./frontend.md)
7. [`guide/data-and-sync.md`](./data-and-sync.md)
8. [`guide/troubleshooting.md`](./troubleshooting.md)
9. [`guide/live-release-checklist.md`](./live-release-checklist.md) when moving work from `development` to the live app
10. [`guide/stable-release-workflow.md`](./stable-release-workflow.md) for release setup and troubleshooting

If you only need one area:

- changing Go code: start with [`guide/backend.md`](./backend.md)
- changing React code: start with [`guide/frontend.md`](./frontend.md)
- changing database or sync rules: start with [`guide/data-and-sync.md`](./data-and-sync.md)
- fixing a real bug or user incident: start with [`guide/troubleshooting.md`](./troubleshooting.md)

## Document map

| File | What it helps you understand |
| --- | --- |
| `guide/README.md` | High-level orientation, mental model, and reading order |
| `guide/takeover-playbook.md` | Practical first-day and first-week takeover plan |
| `guide/architecture.md` | Runtime boundaries, app startup, shutdown, state ownership, and system flow |
| `guide/backend.md` | Go packages, Wails facade, services, repositories, sync engine, integrations |
| `guide/frontend.md` | React app structure, state management, pages, event subscriptions, UI caveats |
| `guide/data-and-sync.md` | Local schema, record lifecycle, sync states, screenshot pipeline, debugging data issues |
| `guide/developer-workflow.md` | Setup, commands, safe change patterns, testing, release workflow |
| `guide/troubleshooting.md` | Incident response, debugging checklists, common failure modes |
| `guide/glossary.md` | Plain-language definitions of project and technical terms |
| `guide/stable-release-workflow.md` | Development-to-main merge, tag-triggered live release, and update verification |
| `guide/live-release-checklist.md` | Copy-pasteable development-to-main-to-live release checklist |

## Very short repo map

```text
wails-app/
|-- backend/             Go application code
|   |-- app/             Wails-facing App facade and desktop-specific logic
|   |-- db/              database bootstrap, migrations, schema repair, indexes
|   |-- db/models/       GORM models and exported data shapes
|   |-- repository/      direct database access
|   |-- service_classes/ main application orchestration layer
|   |-- services/        mixed helpers, integrations, and older still-used logic
|   |-- tests/           selected backend tests
|   `-- utils/           logging, recovery, correlation helpers
|-- frontend/            React + TypeScript application
|   |-- src/             pages, components, hooks, store, styles
|   `-- wailsjs/         generated bridge files for the Go <-> TS contract
|-- guide/               this documentation set
|-- plan/                planning notes, not runtime code
|-- main.go              process entry point
|-- wails.json           Wails build/package metadata
`-- go.mod               Go module and backend dependency list
```

## The runtime, in plain language

Here is the fastest accurate mental model of the running app:

1. Wails launches a desktop window.
2. The React app renders inside that window.
3. The React app calls generated functions that represent Go methods.
4. Those Go methods live on `backend/app.App`.
5. `App` calls service objects that implement actual business behavior.
6. Services read and write local SQLite records.
7. Some services also call the remote FES API.
8. Some services also touch the file system, screenshot capture libraries, the OS keyring, or Backblaze cloud storage.
9. Background work emits runtime events back to the frontend so the UI can refresh itself.

## Current code realities a new maintainer should know

These are not complaints. They are realities you should know before refactoring.

### 1. The app is layered, but not perfectly pure

There is a clear intended architecture:

- `backend/app` = frontend-facing facade
- `backend/service_classes` = use-case orchestration
- `backend/repository` = persistence layer
- `backend/db` = database bootstrap and migrations

But `backend/services` is still very relevant. It contains:

- screenshot storage helpers
- activity log queries
- summary logic
- Backblaze upload support
- some login and attendance helper paths
- other transitional logic

Do not assume "`service_classes` is new and `services` is dead". Read call sites first.

### 2. Offline-first design changes everything

The app must continue to work when the network is unavailable.

That means:

- records are written locally first
- remote sync may happen later
- a completed UI action does not always mean the remote system was updated yet
- sync metadata matters for recovery

### 3. Attendance is a dependency, not just a feature

The app cannot fully sync a task record until it has enough attendance state to identify the remote session. In practice, task syncing often depends on the attendance record having a remote `login_logout` identifier first.

### 4. Screenshots are a full subsystem

Screenshot support is not one flag and one file. It spans:

- frontend release flags
- task tracking behavior
- screenshot capture
- local file storage
- screenshot rows in SQLite
- sync reconciliation
- Backblaze upload

At the moment, the frontend release flag and the backend enable constants are all enabled, so screenshot behavior is intended to be active. If you disable or redesign it, align every layer.

### 5. Version values are split across files

At the time of writing, version values are not fully synchronized:

- `backend/app/app.go` has `CurrentVersion = "2.4.5"`
- `wails.json` has `"productVersion": "2.4.4"`
- `frontend/src/components/custom/AppFooterInfo.tsx` has a fallback default of `2.1.0`

For release work, update all version sources together.

### 6. Firebase exists, but is not the whole auth story

The frontend initializes Firebase and listens to Firebase auth state, but the main operational login flow is handled by Go against the FES API. That means you should be careful when someone says "auth is in Firebase" or "auth is in Go". In this repo, both appear in the code, but the practical session workflow is mostly FES-driven.

## First-day checklist

If you are taking over the project, do this on day 1:

1. Run the app with `wails dev`.
2. Log in using a safe development account if one exists.
3. Visit the main screens:
   - login
   - home
   - one task page
   - activity logs
   - account settings
4. Read `main.go` and `backend/app/app.go`.
5. Read `frontend/src/App.tsx` and `frontend/src/store/AuthContext.ts`.
6. Trace one full action end-to-end, preferably start tracking or stop tracking.
7. Read the sync guide before changing anything related to task or attendance behavior.

## How to think like a safe maintainer in this codebase

When you touch code, ask these questions:

1. Does this still work if the user is offline?
2. If remote sync fails, is the local record still correct?
3. Does the frontend expectation still match the Go method shape?
4. Does this change need database migration work too?
5. Does this impact screenshots, task tracking, or attendance ordering?
6. Does a runtime event need to be emitted so the UI updates?
7. Does a generated Wails contract need regeneration?

## What not to do casually

- Do not delete or rewrite sync-state columns because they look noisy.
- Do not hand-edit generated files under `frontend/wailsjs/`.
- Do not assume the local database can be recreated without user impact.
- Do not move code between `services` and `service_classes` just to make the folders "look cleaner".
- Do not change attendance/task ordering rules without understanding sync consequences.

## Best code-reading pattern for a "vibe coder"

If you are more comfortable learning by tracing behavior than by reading architecture docs first, use this pattern:

1. Start from the visible screen.
2. Find the button or effect that triggers the behavior.
3. Find the imported Wails binding it calls.
4. Find the matching method on `backend/app.App`.
5. Find the service method called by `App`.
6. Find the repository methods and database writes underneath it.
7. Find any runtime events emitted back to the frontend.

That pattern works for almost every important user flow in the app.

## Next step

If you want the fastest useful next read, go to [`guide/takeover-playbook.md`](./takeover-playbook.md). It turns the rest of this documentation set into a practical first-week plan.
