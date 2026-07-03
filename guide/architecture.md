# Architecture

This document explains how the application is assembled at runtime and how data moves through the system.

## 1. System context

FES App Tracker is a desktop application built with Wails. Wails embeds a web frontend inside a native desktop shell and lets the frontend call Go methods directly.

The major runtime parts are:

```text
Desktop shell
  Wails window
    React frontend
      generated Wails bindings
        Go App facade
          application services
            local SQLite database
            external FES API
            optional Backblaze screenshot storage
```

## 2. Runtime boundaries

### Desktop boundary

Files:

- `main.go`
- `backend/app/app.go`
- `wails.json`

Responsibilities:

- start Wails
- embed frontend assets for production
- configure the window
- enforce single-instance behavior
- bind the Go `App` struct so frontend code can call its methods
- manage startup and shutdown hooks

### Frontend boundary

Files:

- `frontend/src/main.tsx`
- `frontend/src/App.tsx`
- `frontend/src/pages/**`
- `frontend/src/store/AuthContext.ts`

Responsibilities:

- render the UI
- route between screens
- hold short-lived client state with Zustand
- call Go bindings
- subscribe to backend-emitted runtime events

### Backend facade boundary

File:

- `backend/app/app.go`

Responsibilities:

- expose methods to the frontend
- hold session-scoped runtime state in memory
- coordinate service objects
- normalize errors before they cross into the UI
- trigger OS-level features like updates, tray behavior, and system integration

### Persistence boundary

Files:

- `backend/db/**`
- `backend/repository/**`

Responsibilities:

- open SQLite
- run migrations and schema repair
- persist users, attendances, tasks, task records, screenshots
- provide data access primitives used by services

### External integration boundary

Files:

- `backend/service_classes/fes_client.service.go`
- `backend/services/backblaze.go`

Responsibilities:

- talk to the FES API
- retrieve remote credentials
- sync attendance and productivity records
- optionally upload screenshots

## 3. Boot sequence

The normal startup flow is:

1. `main.go` loads `.env` through `godotenv.Load()`.
2. `main.go` decides whether the process is development mode by checking `DEVELOPMENT`.
3. `main.go` creates `app.NewApp()`.
4. Wails starts and calls `App.Startup(ctx)`.
5. `App.Startup`:
   - stores the Wails `context.Context`
   - connects `TaskService` to that context
   - configures file logging
   - initializes the SQLite database
   - starts the tray icon and tray menu loop
6. The frontend renders and begins calling Go methods through `frontend/wailsjs`.

## 4. Shutdown sequence

`App.Shutdown(ctx)` performs application cleanup:

1. stop screenshot capture worker if running
2. force connectivity state to offline
3. if a task is active, stop local tracking so the record becomes retryable
4. stop the sync queue worker
5. quit the tray process

This design is important because the app assumes the local database should survive abnormal or imperfect network conditions.

## 5. Layer model

The intended backend layering looks like this:

```text
App facade
  -> service_classes
    -> repository
      -> db/models + GORM + SQLite
```

The actual codebase also includes a parallel `backend/services` package that plays several roles:

- domain helpers
- date/time formatting helpers
- storage helpers
- external integration support
- older compatibility login/attendance/task functions

That means the real architecture is closer to:

```text
App facade
  -> service_classes
       -> repository
       -> selected helpers in backend/services
  -> selected direct calls into backend/services
```

This is not necessarily wrong, but it is important. A new developer should not assume the package name alone tells you whether code is current, legacy, or utility-only.

## 6. Frontend to backend call flow

Most interactive behavior uses this path:

1. A React page or component imports a generated function from `frontend/wailsjs/go/app/App`.
2. The component calls that function like a normal async TypeScript function.
3. Wails forwards the call to the exported Go method on `backend/app.App`.
4. `App` delegates to one or more backend services.
5. Services use repositories or integration clients.
6. Data or errors return back to the frontend promise.

Example:

```text
Task.tsx
  -> StartTracking(taskId, folderName, isPadagan)
    -> App.StartTracking(...)
      -> TaskService.StartTracking(...)
        -> TaskRepository writes task_record
        -> ScreenshotService starts worker
        -> SyncService attempts remote sync
```

## 7. Event-driven updates

Not everything is request/response. Several backend operations emit Wails runtime events that the frontend listens for.

Common examples:

- `employee_details_fetched`
- `record-synced`
- `attendance-synced`
- `tasks-sync-warning`
- `screenshot`
- `upwork-button`

This creates a hybrid architecture:

- direct method calls for commands and queries
- event subscriptions for background updates and asynchronous completion signals

## 8. Session state model

There are three state layers to understand:

### In-memory frontend state

Held in Zustand:

- current user info
- online/offline status
- current task id
- loaded task list
- summaries
- current attendance summary state

### In-memory backend state

Held in `backend/app.App`:

- `AuthUser`
- `Attendance`
- `AttendanceID`
- `WorkingTaskID`
- `WorkingTask`
- `ScreenshotService`
- `SyncService`

### Durable local state

Stored in SQLite:

- users
- attendances
- tasks
- task_records
- screenshots

The durable local state is the real operational record. Both frontend and backend memory state can be reconstructed from it after restart.

## 9. Online and offline architecture

The app is designed as a local-first desktop tracker.

### Offline mode

- user can log in only if an account already exists locally
- attendance is created locally
- task records are created locally
- sync attempts are blocked
- records stay in pending, failed, or blocked states

### Online mode

- login is validated against the FES API
- remote employee details are fetched
- tasks and clients can be refreshed from the remote source
- attendance and task records are pushed upstream
- pending local records can be retried

## 10. Synchronization architecture

Sync is implemented as a queued background system, not a single direct HTTP call.

`SyncService` is responsible for:

- queueing sync jobs
- deduplicating jobs by key
- retrying transient failures
- preventing sync while offline
- marking local records with state transitions
- enforcing attendance-before-task ordering
- draining screenshot backlogs when possible

This matters because a UI action such as "stop tracking" is only the beginning of the lifecycle. The local record can still be unsynced, blocked, or partially synchronized afterward.

## 11. Data ownership and source of truth

A practical way to think about data ownership:

- React owns presentation-only state.
- `App` owns runtime wiring and session state.
- SQLite owns durable operational data.
- The FES API owns authoritative remote attendance/productivity records once sync succeeds.

When debugging, first decide which layer is currently authoritative for the problem you are looking at.

## 12. Generated contract layer

Files:

- `frontend/wailsjs/go/app/App.d.ts`
- `frontend/wailsjs/go/app/App.js`
- `frontend/wailsjs/go/models.ts`

These are generated by Wails and represent the contract between the frontend and backend.

Rules:

- do not edit them by hand
- when you add or change exported Go methods or model shapes, regenerate by running the Wails workflow
- treat them like compiled interface artifacts

## 13. Desktop-specific subsystems

Several parts of the app are desktop concerns rather than normal web-app concerns:

- system tray support via `systray`
- single-instance lock
- window hide-on-close behavior
- local file system storage
- native keyring access
- launching installers
- opening external URLs and custom protocols

This means some behavior is only meaningful inside the Wails runtime and cannot be fully understood by reading the frontend alone.

## 14. Current architectural debt and caution areas

These are not necessarily bugs, but they are the places where maintainers should slow down:

### Mixed service boundaries

Business logic is split between `service_classes` and `services`. Do not refactor package boundaries casually.

### Feature-flag inconsistency

Frontend and backend screenshot flags disagree. Release behavior should be aligned before expanding screenshot work.

### Version-source inconsistency

There are multiple version declarations. Release changes should update all of them together.

### Authentication ambiguity

Firebase presence suggests one auth model, but the main functional login path uses the FES API. Document your assumptions before altering auth-related code.

### Context-sensitive runtime behavior

Some logic changes behavior based on Wails build type or the `DEVELOPMENT` environment variable. Storage paths, instance identifiers, and timing behavior can differ between dev and production.

## 15. Read next

- `guide/backend.md` for the Go implementation
- `guide/frontend.md` for the React implementation
- `guide/data-and-sync.md` for the record lifecycle and schema details
