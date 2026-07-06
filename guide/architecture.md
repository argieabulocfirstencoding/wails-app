# Architecture

This document explains how the application is assembled, how the layers talk to each other, and where the important boundaries are.

If you are new to desktop apps, read this slowly. Most maintenance mistakes in this project happen because a developer changes one layer while assuming another layer works like a normal web app.

## 1. The big-picture shape

The app is a desktop shell that embeds a web frontend and a Go backend in the same installed application.

That sentence carries a lot of meaning:

- "desktop shell" means it can integrate with the system tray, local files, and the OS
- "web frontend" means the UI is built with React and TypeScript
- "Go backend" means business logic is not limited to the browser side
- "same application" means the frontend and backend communicate through Wails, not by calling a separately deployed HTTP server running on your machine

At runtime, the system looks like this:

```text
Windows desktop app
  -> Wails runtime
    -> WebView window
      -> React frontend
        -> generated Wails bindings
          -> backend/app.App
            -> service classes
              -> repositories
                -> SQLite database
            -> selected helper/integration services
              -> FES API
              -> file system
              -> OS keyring
              -> Backblaze B2
```

## 2. The main boundaries

Thinking in boundaries helps prevent accidental architectural damage.

### Boundary A: process startup and desktop runtime

Main files:

- `main.go`
- `wails.json`
- `backend/app/app.go`

This boundary is responsible for:

- starting the Wails application
- embedding the built frontend for production
- creating the application window
- binding the Go `App` struct so the frontend can call it
- enforcing the single-instance lock
- handling startup and shutdown hooks

### Boundary B: frontend UI and local page behavior

Main files:

- `frontend/src/main.tsx`
- `frontend/src/App.tsx`
- `frontend/src/pages/**`
- `frontend/src/store/AuthContext.ts`

This boundary is responsible for:

- rendering the user interface
- routing between screens
- holding short-lived UI state
- responding to runtime events from the Go backend
- deciding when to call Wails-exposed methods

### Boundary C: Wails-facing backend facade

Main file:

- `backend/app/app.go`

This boundary is responsible for:

- exposing methods to the frontend
- keeping session-scoped in-memory state
- calling deeper services
- normalizing backend failures into safer frontend-facing errors
- handling desktop-specific capabilities such as updates and tray behavior

### Boundary D: application orchestration

Main folder:

- `backend/service_classes/`

This boundary is where most real use-case logic lives:

- authentication
- attendance creation
- task tracking
- synchronization
- screenshot capture orchestration
- connectivity state

### Boundary E: persistence

Main folders:

- `backend/repository/`
- `backend/db/`
- `backend/db/models/`

This boundary is responsible for:

- opening SQLite
- applying migrations
- keeping schema and indexes healthy
- reading and writing local records

### Boundary F: external integrations

Main files:

- `backend/service_classes/fes_client.service.go`
- `backend/services/backblaze.go`
- `backend/services/storage.go`

This boundary is responsible for:

- remote FES API requests
- OS keyring access
- screenshot upload
- file-system storage paths

## 3. Startup flow

Understanding startup matters because some bugs only exist before the user clicks anything.

### Step-by-step startup

1. `main.go` loads environment variables with `godotenv.Load()`.
2. `main.go` checks whether `DEVELOPMENT` exists in the environment.
3. Based on that flag, it changes the app title and the single-instance unique ID.
4. `main.go` creates a new `app.App` by calling `app.NewApp()`.
5. Wails starts with that `App` instance bound into the runtime.
6. Wails calls `App.Startup(ctx)`.
7. `App.Startup`:
   - stores the Wails `context.Context`
   - connects the task service to that context
   - configures file logging
   - initializes SQLite
   - launches the system tray loop

### Why the Wails context matters

Several services need the Wails runtime context because they use it to:

- emit runtime events back to the frontend
- inspect whether the build is `dev` or `production`
- call Wails window APIs
- write logs through Wails runtime helpers

If a function needs to emit `runtime.EventsEmit(...)`, it usually needs access to that context somewhere in the call chain.

## 4. Shutdown flow

Shutdown is not just cleanup. It is also part of data safety.

`App.Shutdown(ctx)` currently does all of this:

1. stops the screenshot worker
2. marks connectivity as offline
3. if a task is active, stops tracking so the local record closes cleanly
4. shuts down the sync queue
5. quits the tray process

This is important because the app tries to leave local work in a retryable state instead of silently abandoning it.

## 5. The most important architectural rule: local-first state

The safest way to understand the application is:

- the remote FES API is important
- the local database is still the day-to-day operational source of truth

Why?

Because the user may:

- lose connectivity
- close the app mid-session
- restart after a partial failure
- have pending sync work left over from an earlier session

So the app must remember more than "what just happened in the UI".

## 6. State lives in three places

This is one of the most important concepts for a new maintainer.

### Frontend memory state

Held mainly in the Zustand store at `frontend/src/store/AuthContext.ts`.

Examples:

- current user object
- online/offline flag
- selected task ID
- cached task list
- summary totals

This state is convenient, but it is not durable.

### Backend memory state

Held mainly on `backend/app.App`.

Examples:

- `AuthUser`
- `Attendance`
- `AttendanceID`
- `WorkingTaskID`
- pointers to the active service objects

This state lasts only as long as the current app process.

### Durable local state

Stored in SQLite.

Examples:

- users
- attendances
- tasks
- task records
- screenshots
- sync status metadata

This is the state that survives restart and makes offline-first behavior possible.

## 7. How the frontend talks to the backend

This is not a REST API architecture. The browser is not sending fetch requests to local endpoints for core actions.

The actual pattern is:

1. A React file imports a generated function from `frontend/wailsjs/go/app/App`.
2. The function is called like an async TypeScript function.
3. Wails forwards that call into the matching Go method on `backend/app.App`.
4. That Go method returns data or an error back through the bridge.

Example:

```text
frontend/src/pages/Main/Task.tsx
  -> StartTracking(...)
    -> App.StartTracking(...)
      -> TaskService.StartTracking(...)
        -> TaskRepository + ScreenshotService + SyncService
```

### Why this matters

If the frontend appears to "call the backend", you usually need to inspect:

- the generated binding import
- the `App` method
- the service method behind it

## 8. Commands versus events

The app uses two communication styles.

### Style A: direct command/query calls

Examples:

- `StartTracking`
- `StopTracking`
- `GetTasks`
- `GetCurrentAttendance`

The frontend calls something and waits for the result.

### Style B: runtime events

Examples emitted by the backend:

- `employee_details_fetched`
- `record-synced`
- `attendance-synced`
- `tasks-sync-warning`
- `screenshot`
- `upwork-button`

The frontend subscribes with `EventsOn(...)` and reacts later.

### Why this distinction matters

Many background jobs finish after the original button click has already completed. If the UI looks stale, you need to inspect both:

- the original command
- the later event that tells the frontend to refresh

## 9. The actual backend layering

The ideal architecture is roughly:

```text
App facade
  -> service classes
    -> repositories
      -> models + GORM + SQLite
```

The real architecture is slightly messier:

```text
App facade
  -> service classes
       -> repositories
       -> selected helpers in backend/services
  -> selected direct calls into backend/services
```

This is normal for a living application, but it means you should verify usage before refactoring package boundaries.

## 10. The `App` facade is a wiring hub

`backend/app/app.go` is more than a random file. It is the main backend object the frontend can reach.

It does several things:

- stores the Wails context
- owns runtime-scoped state like the current attendance and current task
- wires service objects together in `NewApp()`
- exposes Wails-callable methods
- performs desktop tasks like updates and tray behavior

### Why the pointer wiring matters

`NewApp()` passes pointers around so multiple services can share the same live state.

Examples:

- the task service gets a pointer to `App.Attendance`
- the sync service gets a pointer to the active task record
- the screenshot service is connected back to the sync service

This keeps layers coordinated, but it also creates subtle coupling. Refactoring these pointers without understanding their role can cause bugs that are hard to spot.

## 11. Detailed example: start tracking a task

This is the best single flow to understand because it touches almost every layer.

### User-visible action

The user opens a task page and presses Start.

### Frontend side

`frontend/src/pages/Main/Task.tsx` calls:

- `StartTracking(id, JSON.stringify(timeStart), isPadagan)`

It then updates the local page state to show the timer as running.

### Wails bridge side

The generated binding forwards the call to:

- `App.StartTracking(taskId, localStorageValue, padagan)`

### Backend facade side

`App.StartTracking` calls:

- `AuthService.TaskService.StartTracking(...)`

### Service side

`TaskService.StartTracking` then:

1. verifies there is an active attendance
2. defensively stops lingering active records
3. may try to sync the previous record if one exists
4. creates a new local task record
5. stores task and record pointers in memory
6. looks up the task to see whether it is a break task
7. starts the screenshot worker for non-break tasks
8. asks the sync service to sync the task record

### Data side

The new task record is inserted into SQLite before remote sync is guaranteed.

### Remote side

If online and prerequisites are satisfied, the sync service attempts the remote task-in request.

## 12. Detailed example: stop tracking a task

This flow is equally important because it turns an active record into a completed unit of work.

`TaskService.StopTracking` does all of this:

1. stops screenshot capture
2. closes the active local task record
3. resets sync state so the record is pending
4. if online, asks `SyncService` to sync task-out
5. clears active in-memory task pointers

Notice the distinction:

- the local task is stopped immediately
- the remote task-out may succeed later

That is classic local-first behavior.

## 13. Synchronization is a background queue, not a single call

One of the most important architectural facts is that sync lives in `backend/service_classes/sync.service.go` as a queued, retrying subsystem.

The sync service currently does all of this:

- queues jobs
- deduplicates jobs by key
- retries retryable failures with backoff
- blocks sync while offline
- updates local sync-state fields
- enforces attendance-before-task ordering
- drains pending screenshot uploads after task sync
- emits frontend events about success or failure

So when the user says "I clicked stop", that is only the beginning of the sync lifecycle.

## 14. Attendance is an upstream dependency

Task syncing depends on attendance syncing first.

In plain language:

- the remote system needs to know which attendance session the task belongs to
- that remote attendance identity is stored in `attendance.LoginLogoutID`
- if the task record tries to sync before that exists, the app delays or blocks task sync

This rule is enforced inside `SyncService.ensureAttendanceReadyForTaskSync(...)`.

If you break this invariant, you will create hard-to-debug partial sync failures.

## 15. Screenshots are not just UI decoration

The screenshot subsystem crosses architectural boundaries:

- task page UI displays latest screenshot
- screenshot service captures active displays
- files are stored on disk
- metadata is stored in SQLite
- sync service may upload pending screenshot files after task sync
- Backblaze stores remote copies

The app therefore treats screenshots as a real persistence and synchronization feature, not just a temporary preview.

## 16. Storage and secret boundaries

Not all state belongs in SQLite.

### SQLite stores

- user profile cache
- attendance records
- tasks and clients
- task records
- screenshot metadata
- sync attempt metadata

### File system stores

- actual screenshot image files
- log files
- diagnostics artifacts

### OS keyring stores

- FES API token
- Backblaze credentials

This separation matters when debugging. If a cloud upload is failing, the problem might be:

- a screenshot database row
- a missing local file
- a missing keyring credential
- a remote API issue

## 17. Build-mode differences

The application changes behavior depending on development versus production runtime.

Examples:

- unique app ID changes
- window title changes
- database filename changes
- screenshot storage location changes
- some behavior uses runtime build type checks

The environment switch is based mainly on the `DEVELOPMENT` environment variable and Wails runtime environment checks.

## 18. Generated contract layer

Wails creates generated bridge files under `frontend/wailsjs/`.

These files are important because they are the formal contract between the Go side and the TypeScript side.

Rules:

- do not edit them by hand
- if you add or change an exported `App` method, regenerate them through the Wails workflow
- if a Go model changes shape and is exposed to the frontend, the generated TypeScript must be refreshed too

## 19. Architecture risks and debt to respect

These are the places where an experienced maintainer slows down before making a "simple" cleanup.

### Mixed service boundaries

Not every file in `backend/services/` is legacy, and not every file in `service_classes/` is the only modern path.

### Shared state by pointer

Several services coordinate through shared pointers to current attendance, task record, and working task ID.

### Auth ambiguity

Firebase exists in the frontend, but the main operational login path is FES API based.

### Version-source mismatch

Version values currently live in multiple places and are not fully aligned.

### Screenshot complexity

The screenshot feature spans UI, storage, sync, and cloud upload. It is easy to break one part while changing another.

## 20. What should stay true after any refactor

If you are changing architecture or moving code, preserve these outcomes:

1. The app can still work offline.
2. Local data is still enough to recover and retry sync.
3. Attendance must still be ready before dependent task sync completes.
4. The frontend must still receive the events it relies on.
5. Screenshot files and screenshot metadata must remain consistent.
6. Desktop-only features must still run inside the Wails runtime.

## Read next

- [`guide/backend.md`](./backend.md) for the Go implementation details
- [`guide/frontend.md`](./frontend.md) for the React implementation details
- [`guide/data-and-sync.md`](./data-and-sync.md) for the data lifecycle and sync state machine
