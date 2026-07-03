# Developer Guide

This folder is the main onboarding and maintenance guide for the current `wails-app` codebase.

If you are new to the project, read these files in order:

1. `guide/README.md`
2. `guide/architecture.md`
3. `guide/developer-workflow.md`
4. The backend or frontend deep dive that matches the area you need to change

## Why this guide exists

The top-level repository structure is simple, but the implementation has several moving parts:

- A Go desktop runtime built with Wails
- A React frontend that talks to Go through generated bindings
- A local SQLite database used as the operational source of truth
- A synchronization pipeline that pushes attendance and task records to the FES API
- Optional screenshot and cloud-upload infrastructure that is partially disabled in the current release

This guide is meant to help a new developer understand how the app really works today, not how it was originally intended to work.

## Fast orientation

At a high level, the runtime looks like this:

```text
React UI
  -> frontend/wailsjs generated bindings
    -> backend/app.App
      -> service_classes
        -> repository
          -> GORM
            -> SQLite

backend/app.App
  -> services/FesClientService
    -> FES API

backend/app.App
  -> services/BackblazeSync
    -> Backblaze B2
```

The most important mental model is:

- The frontend does not call HTTP endpoints directly for core app behavior.
- The frontend calls exported Go methods from `backend/app.App`.
- `App` is the Wails-facing facade, not the place where most business rules should live.
- The local database is the durable record used during offline operation and during sync retries.
- Sync is not instant or guaranteed. Records move through sync states and may need retries.

## What each guide covers

| File | Purpose |
| --- | --- |
| `guide/architecture.md` | System boundaries, request flow, runtime layers, boot/shutdown behavior, and major architectural decisions |
| `guide/backend.md` | Go backend structure, Wails bindings, services, repositories, migrations, sync engine, updates, diagnostics |
| `guide/frontend.md` | React entry points, routing, Zustand store, event subscriptions, page responsibilities, frontend caveats |
| `guide/data-and-sync.md` | SQLite schema, sync state machine, record lifecycle, screenshot storage, cloud sync behavior, troubleshooting |
| `guide/developer-workflow.md` | Local setup, daily workflow, commands, change patterns, testing, release checklist, debugging |
| `guide/glossary.md` | Technical terms used throughout the app and codebase |

## Current implementation realities

These are the first things a new maintainer should know because they affect almost every change:

### 1. The architecture is partially layered, partially transitional

There is a clear intended layering:

- `backend/app` = Wails application facade
- `backend/service_classes` = application services and orchestration
- `backend/repository` = persistence layer
- `backend/db` = schema, migrations, database bootstrap

But `backend/services` is mixed:

- Some files are still active and important, such as activity log logic, storage helpers, and Backblaze support.
- Some files are older compatibility logic that overlaps with repository or service-class behavior.
- You should inspect call sites before refactoring anything in `backend/services`.

### 2. Offline-first behavior is a core design constraint

The app is built around local persistence:

- login can work offline if the user already exists locally
- attendance and task records are written to SQLite first
- sync to the FES API happens afterward
- failed sync attempts are tracked and can be retried

If you change business logic, ask yourself:

- does it still work when offline?
- is the local record still valid if remote sync fails?
- will a later retry still have enough data to finish?

### 3. Attendance sync and task sync are coupled

Task records depend on attendance state. A task cannot fully sync until the related attendance session has a remote `login_logout` identity or an already-synced state. This dependency is enforced in `SyncService`.

### 4. Screenshot support is inconsistent across layers

Current code reality:

- `frontend/src/config/release.ts` has `screenshotsEnabled: true`
- `backend/service_classes/screenshot.service.go` has `const screenshotsEnabled = false`
- `backend/services/backblaze.go` has `const backblazeEnabled = false`

That means:

- frontend UI may behave as if screenshot support exists
- backend capture and upload services are disabled
- any screenshot-related work should start by deciding which layer is the real source of truth

### 5. Version values are not fully synchronized

Current version values come from multiple files:

- `backend/app/app.go` sets `CurrentVersion = "2.4.5"`
- `wails.json` says `productVersion = "2.4.4"`
- the old README also referenced `2.4.4`
- `AppFooterInfo` has a fallback default of `2.1.0`, although it fetches the runtime version later

For release work, update all version sources together.

### 6. Firebase is present, but it is not the core login path

The frontend initializes Firebase and subscribes to `onAuthStateChanged`, but the primary online login flow is:

`Login.tsx` -> `LoginUserOnline(...)` -> FES API via Go backend

Treat Firebase-related code carefully. Some of it looks foundational, but the present authentication flow is driven by the FES backend integration and local session state.

## Repo map

```text
wails-app/
|-- backend/            Go application code
|   |-- app/            Wails-bound App facade and desktop-specific behavior
|   |-- db/             database bootstrap, migrations, schema repair, indexes
|   |-- db/models/      GORM models and frontend export shapes
|   |-- repository/     database access layer
|   |-- service_classes/ current orchestration layer
|   |-- services/       mixed utility, integration, and older business logic
|   |-- tests/          some focused backend tests
|   `-- utils/          logging, panic recovery, correlation helpers
|-- frontend/           React + TypeScript UI
|   |-- src/            app code
|   |-- wailsjs/        generated bindings, do not edit manually
|   `-- package.json    frontend dependencies and scripts
|-- build/              packaging assets and Wails build outputs
|-- guide/              this documentation set
|-- plan/               planning artifacts, not part of runtime behavior
|-- main.go             Wails process entry point
|-- wails.json          Wails configuration and package metadata
`-- README.md           short project entry point
```

## Suggested learning path for a new developer

If you need to become productive quickly, follow this path:

1. Run the app with `wails dev`.
2. Read `main.go` and `backend/app/app.go` to understand the Wails boundary.
3. Read `frontend/src/App.tsx` and `frontend/src/store/AuthContext.ts` to understand routing and state.
4. Trace one full feature from UI to DB, such as starting a task:
   `frontend/src/pages/Main/Task.tsx` -> `StartTracking` binding -> `TaskService.StartTracking` -> repository writes -> `SyncService`.
5. Read `guide/data-and-sync.md` before changing anything related to attendance, task recording, or retries.

## Ground rules before you change code

- Do not edit files under `frontend/wailsjs/` by hand.
- Assume the local SQLite database is important and may contain recoverable user data.
- Be cautious when changing sync status logic, because retries and offline behavior depend on it.
- Inspect both `backend/service_classes` and `backend/services` before removing "duplicate-looking" code.
- Check whether the change must also update frontend expectations, generated types, migrations, or release flags.

## Next step

Continue with:

- `guide/architecture.md` for the system-wide picture
- `guide/backend.md` if you are changing Go code
- `guide/frontend.md` if you are changing React code
- `guide/developer-workflow.md` if you are preparing your environment or planning a release
