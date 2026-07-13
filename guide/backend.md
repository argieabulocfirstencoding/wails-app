# Backend Deep Dive

This document explains the Go side of the application in a way that assumes you may be new to Go or new to Wails-backed desktop applications.

If you only read one backend file first, make it `backend/app/app.go`. That file is the center of gravity for how the desktop app exposes behavior to the frontend.

## 1. Backend folder map

```text
backend/
|-- app/                Wails-bound App struct, tray behavior, diagnostics
|-- db/                 database bootstrap, migrations, repair logic, indexes
|-- db/models/          GORM models and exported DTO-like structs
|-- repository/         persistence operations
|-- service_classes/    current orchestration layer
|-- services/           mixed helper and integration layer
|-- tests/              focused backend tests
|-- types/              shared request, error, and summary types
`-- utils/              logging, panic recovery, correlation ids
```

## 2. What "backend" means in this project

In a web app, "backend" often means a separate server process running elsewhere.

In this project, the backend is the Go code running inside the desktop application process itself.

That means backend code can:

- talk to the local SQLite database
- read and write local files
- use screenshot and system-tray libraries
- call remote APIs
- store tokens in the OS keyring
- emit runtime events to the frontend

It is both business layer and desktop-integration layer.

## 3. Entry point: `main.go`

Files:

- `main.go`
- `backend/app/app.go`

`main.go` is small, but it matters because it sets up the runtime contract.

It currently does these important things:

- loads `.env`
- checks `DEVELOPMENT`
- creates `app.NewApp()`
- binds that `App` instance into Wails
- configures window size, single-instance lock, asset server, and shutdown hooks

The app window is configured here too, including:

- width and height
- background color
- hide-on-close behavior
- disabled resize

## 4. The `App` struct is the backend facade

File:

- `backend/app/app.go`

`App` is the object Wails exposes to the frontend. When the frontend imports generated functions like `StartTracking` or `GetTasks`, those map back to methods on this struct.

### Major responsibilities of `App`

It has several jobs:

1. hold runtime state for the current desktop session
2. expose methods to the frontend through Wails
3. wire service instances together

### Important fields on `App`

- `ScreenshotService`
- `FirebaseSyncService`
- `AuthService`
- `SyncService`
- `Connectivity`
- `AuthUser`
- `Attendance`
- `AttendanceID`
- `WorkingTaskID`

### Wails-exposed method groups

#### Startup and shutdown

- `Startup`
- `Shutdown`
- `OnSecondInstanceLaunch`

#### Authentication and session

- `LoginUser`
- `LoginUserOnline`
- `FetchUserDetails`
- `LogoutUser`
- `GetAccounts`

#### Task tracking

- `StartTracking`
- `StopTracking`
- `GetActiveTaskRecord`
- `GetLastTaskRecord`
- `GetLastTaskRecords`
- `SetTaskRecordNotes`
- `CreateTask`
- `GetTasks`

#### Attendance and activity

- `GetCurrentAttendance`
- `GetActivityLogRecord`
- `GetTotalClientRecords`

#### Sync and recovery

- `SyncAttendance`
- `SyncUnsyncedAttendance`
- `SyncUnsyncedRecords`
- `SyncUnsyncedTaskRecord`
- `SyncAttendanceByID`
- `FetchActiveTask`

#### Desktop and support features

- `GetMonitorCount`
- `GetFolderSize`
- `ClearScreenshots`
- `ExportDiagnosticBundle`
- `CheckForUpdates`
- `DownloadAndInstall`
- `OpenUpwork`
- `OpenFesWebApp`
- `GetAppCurrentVersion`

### Important facade design rule

`App` should remain a facade. If you are adding a feature and the method starts becoming large, move decision logic into a service and keep the Wails method focused on:

- auth/session guards
- parameter handoff
- error normalization
- event orchestration

## 5. How service wiring works in `NewApp()`

`NewApp()` is a very important function because it builds the object graph.

It creates:

- connectivity state service
- Firebase sync service
- logger
- screenshot service
- sync service
- auth service

Then it cross-connects them.

Examples:

- `authService.SyncService = syncService`
- `authService.AttendanceService.SyncService = syncService`
- `authService.TaskService.ScreenshotService = screenshotService`
- `screenshotService.SyncService = syncService`
- `syncService.Attendance = &app.Attendance`
- `syncService.TaskRecord = &authService.TaskService.TaskRecord`

### Why this can feel confusing

This project uses pointer sharing so multiple services can operate on the same current session state. That is useful, but it can feel magical if you are expecting strict constructor-injected immutability. Read these assignments carefully before refactoring.

## 6. Startup behavior in the backend

`App.Startup(ctx)` does the real backend initialization work.

Important steps:

1. store the runtime context
2. connect the task service to that context
3. configure file logging
4. initialize the database
5. start the tray icon and tray menu event loop

### Why the database init is early

The app is local-first. Without a working SQLite database, the app cannot safely continue because attendance, tasks, sync retries, and screenshots all depend on durable local storage.

If database initialization fails, the app shows an error dialog and quits.

## 7. Shutdown behavior in the backend

`App.Shutdown(ctx)` is designed to leave local data in a sane state.

It:

- stops screenshot capture
- marks connectivity offline
- stops active tracking if needed
- shuts down the sync queue
- stops the tray process

The important design intention is:

- do not leave a "running task" hanging in memory only
- close the local record so it can be retried later if needed

## 8. Database bootstrap

Files:

- `backend/db/db.go`
- `backend/db/migrations.go`
- `backend/db/schema_tables.go`
- `backend/db/indexes.go`
- `backend/db/backup.go`

### What happens on init

`db.Init(ctx)`:

1. resolves the database path
2. opens SQLite through GORM
3. applies connection settings:
   - `foreign_keys = ON`
   - `journal_mode = WAL`
   - `busy_timeout = 5000`
4. runs schema migrations
5. stores the global `db.DB`

### Database path behavior

`resolveDatabasePath(ctx)` uses `os.UserConfigDir()` and creates a `wails-app` directory under it.

- dev build -> `fesapp-dev.db`
- production build -> `fesapp.db`

### Migration philosophy

This codebase no longer relies on a simple `AutoMigrate` path.

It uses:

- a migration catalog
- schema planning
- canonical table rebuilds when required
- pre-migration backups when destructive repair may occur
- explicit index creation

That is a serious piece of infrastructure. Any schema change should preserve this philosophy.

## 9. Models and exported shapes

There are two kinds of structs you will see here.

### Persistence models

These map to real database tables.

The most important ones are:

- `User`
- `Attendance`
- `Task`
- `TaskRecord`
- `Screenshot`
- `Client`

These models live under `backend/db/models/`.

### Exported or frontend-facing shapes

These are smaller or transformed shapes that are easier to expose to the frontend.

Examples:

- `ExpTask`
- `ExpTaskRecord`
- `ExpAttendance`
- `ActivityLogRecord`
- `TaskRecordWithScreenshots`

These shapes matter because Wails exposes them to TypeScript via generated bindings.

### Why this distinction matters

Not every Go struct should be treated like a raw database row. Some structs exist mainly to shape data for the Wails boundary.

## 10. Repository layer

Files:

- `backend/repository/task.repo.go`
- `backend/repository/attendance.repo.go`
- `backend/repository/auth.repo.go`
- `backend/repository/user.repo.go`

The repository layer encapsulates direct database access. Examples:

- create or update attendances
- query tasks
- stop all active task records
- fetch the latest task record
- create local users

Practical rule:

- if a function is mostly query/update logic against SQLite, it belongs in a repository
- if it coordinates multiple records, services, or remote APIs, it probably belongs above the repository layer

## 11. Why there are both `service_classes` and `services`

### `backend/service_classes/`

This is the current orchestration layer. It contains the main application services:

- `AuthService`
- `AttendanceService`
- `TaskService`
- `SyncService`
- `ScreenshotService`
- `UserService`
- `FesClientService`
- `ConnectivityStateService`

These services are the closest thing to the application's use-case layer.

### `backend/services/`

This package is mixed and should be read carefully before modification. It contains:

- activity log and summary queries
- storage helpers
- Backblaze integration
- old login/attendance helpers still used in some paths
- other utility-style domain functions

Some files are still important and current. Others overlap with the newer layered structure.

### The safe interpretation

Do not treat one folder as "real" and the other as "dead". Use search and call-site tracing before moving or deleting anything.

## 12. Authentication flow

### Online login

Main path:

1. frontend calls `LoginUserOnline(email, password)`
2. `App.LoginUserOnline` calls `AuthService.Login`
3. `AuthService.AttemptLogin` sends `wf/login` to the FES API
4. returned token is stored in keyring and cached in `FesClientService`
5. local user record is created if needed
6. local attendance is created or reused
7. employee details are fetched from the FES API and saved locally
8. sync state is initialized
9. auto sync is kicked off

### Offline login

Main path:

1. frontend calls `LoginUser(email)`
2. Go checks the local `users` table
3. if the email exists locally, a local attendance is created or reused
4. the app marks connectivity offline and continues locally

## 13. `FesClientService` is the remote API bridge

File:

- `backend/service_classes/fes_client.service.go`

This service handles communication with the remote FES API.

Key responsibilities:

- store and retrieve the API token
- make authenticated JSON requests
- fetch Backblaze credentials
- open the FES web app via a magic link
- sync remote clients and projects
- help with Upwork-related desktop automation

### Base URL behavior

It chooses between:

- production base URL
- development base URL

based on the `DEVELOPMENT` environment variable.

### Key operational detail

The API token is cached in memory and also stored in the OS keyring under `fes_api_token`.

## 14. Task service

File:

- `backend/service_classes/task.service.go`

This is the service most developers eventually need to read.

### Responsibilities

- fetch task list
- get latest task records
- create task records
- stop active task records
- manage active tracking lifecycle
- coordinate screenshot capture
- request sync work

## 15. Task tracking flow

Main path for starting work:

1. frontend page calls `StartTracking(taskId, localStorageValue, padagan)`
2. `App.StartTracking` delegates to `TaskService.StartTracking`
3. `TaskService`:
   - ensures attendance exists
   - stops any still-open active records for safety
   - optionally syncs the previous record
   - creates a new `task_records` row
   - stores task/session pointers in memory
   - starts screenshot capture for non-break tasks
   - requests sync of the task record

Stopping work:

1. frontend calls `StopTracking(taskId)`
2. `TaskService.StopTracking` stops screenshot capture
3. it closes the active local task record
4. it marks the record `pending`
5. if online, it attempts remote sync
6. it clears in-memory active-task cache

### Why the `pending` reset matters

Stopping a task locally does not guarantee remote task-out succeeded. Resetting sync state keeps the record retryable.

## 16. Screenshot service

File:

- `backend/service_classes/screenshot.service.go`

This service coordinates local screenshot capture.

### What it currently does

- detects active displays
- captures each display
- encodes the image as WebP
- writes the image to disk
- inserts a `screenshots` row into SQLite
- emits a `screenshot` runtime event
- asks the sync layer to reconcile and upload later when possible

### Important constants

Current code has:

- `const screenshotsEnabled = true`
- production interval = 10 minutes
- development interval = 1 minute

### Storage layout

The service resolves a user-specific screenshot storage root, then creates nested folders based on record/task information. That means screenshot state depends on both database metadata and file-system paths.

## 17. Backblaze upload service

File:

- `backend/services/backblaze.go`

This is the cloud upload service for screenshots.

### What it currently does

- reads Backblaze credentials from the OS keyring
- creates a Backblaze B2 client
- connects to the `fesapp-screenshots` bucket
- uploads screenshot files into remote object paths under `screenshots/<user>/<folder>/...`

### Important operational detail

Current code has `const backblazeEnabled = true`, so upload behavior is intended to be active if credentials are available and the app is online.

### Where credentials come from

The app first asks the FES API for upload credentials, then stores them in the keyring under:

- `APP_ID`
- `APP_KEY`

## 18. Sync service

`backend/service_classes/sync.service.go` is one of the most important files in the codebase.

### What it does

- manages a queue of sync jobs
- deduplicates jobs with keys
- retries transient failures with backoff
- prevents sync while offline
- updates local sync metadata
- ensures task sync waits for attendance readiness
- retries unsynced batches on login or manual actions
- optionally drains screenshot backlogs

### Sync states

The main task and attendance sync states are:

- `pending`
- `in_progress`
- `synced`
- `failed`
- `blocked`

`blocked` means the app intentionally refused to continue due to an unmet prerequisite, most commonly offline mode or an unsatisfied attendance dependency.

### Why attendance must sync first

Task sync depends on a remote `login_logout` identity. `ensureAttendanceReadyForTaskSync` retries attendance sync before allowing remote task sync to continue.

This is one of the most important invariants in the system.

## 19. Connectivity service

File:

- `backend/service_classes/connectivity_state.service.go`

This service is conceptually simple, but it matters because many other services consult it to decide whether they can sync or upload.

It is the backend's notion of online/offline state, which is more important for operational decisions than the browser's own online flag alone.

## 20. Error design

Main file:

- `backend/types/error.type.go`

Two patterns matter most.

### Stable errors

These are user-safe errors intended to cross the Wails boundary without leaking too much internal detail.

### Operation-wrapped errors

Some failures are wrapped with operation and record context so logs are more useful during debugging.

### Practical takeaway

If you return raw internal errors directly to the frontend, you may expose too much detail or create inconsistent UX. Prefer the stable-error path when crossing the Wails boundary.

## 21. Logging and diagnostics

Relevant areas:

- `backend/utils/logger.utils.go`
- `backend/utils/correlation.utils.go`
- `backend/app/diagnostics.go`

The backend includes:

- file logging
- correlation IDs for related operations
- diagnostics bundle export

The diagnostics bundle is especially useful for support because it can collect:

- version and platform metadata
- recent attendance sync states
- recent task record sync states
- the current log file when available

## 22. Update flow

Relevant methods in `backend/app/app.go`:

- `CheckForUpdates`
- `DownloadAndInstall`

The app:

- fetches the latest GitHub release metadata
- validates trusted hosts
- picks an installer asset
- downloads it
- optionally checks a checksum if available
- launches the installer
- quits the app

Important characteristics:

- checks only trusted GitHub hosts
- selects installer assets from release metadata
- validates content type
- optionally verifies checksums if present
- launches the installer and then quits the app

If you touch release or update behavior, inspect both:

- `backend/app/app.go`
- `wails.json`

### Current version mismatch to know about

Current version values are split across files, so a successful update feature still depends on release metadata being maintained carefully.

## 23. Backend tests

The backend test surface is concentrated around risky subsystems:

- migrations
- sync logic
- screenshot record retrieval
- activity log summaries
- attendance guards
- app guard behavior

Useful command:

```bash
go test ./backend/...
```

### Good testing habit in this repo

If you change:

- sync rules
- database schema
- task lifecycle
- screenshot behavior
- startup/shutdown guards

then add or update nearby backend tests instead of relying on manual behavior only.

## 24. Good backend reading order

If you are trying to understand the backend efficiently, read in this order:

1. `main.go`
2. `backend/app/app.go`
3. `backend/service_classes/task.service.go`
4. `backend/service_classes/sync.service.go`
5. `backend/service_classes/auth.service.go`
6. `backend/service_classes/fes_client.service.go`
7. `backend/db/db.go`
8. `backend/db/models/*.go`
9. the relevant repository file

## 25. Safe extension patterns

### If you need a new frontend-callable backend method

1. add a method to `backend/app.App`
2. keep the method thin
3. place real business logic in a service
4. use repositories for data access
5. regenerate Wails bindings
6. update the frontend to call the new binding

### If you need new local data

1. update or add the model
2. update migration logic
3. update repositories
4. update services
5. update exported shapes if the frontend needs the data
6. add tests

## 26. High-risk backend pitfalls

- changing sync state transitions without understanding retry semantics
- deleting "duplicate-looking" code in `backend/services` that is still in use
- forgetting to emit runtime events after background work
- breaking shared pointer assumptions in `App` wiring
- assuming the database is only a cache
- changing screenshot paths without considering local file cleanup and upload correlation

## Read next

- [`guide/data-and-sync.md`](./data-and-sync.md) for schema and lifecycle details
- [`guide/frontend.md`](./frontend.md) to understand how UI actions hit these backend methods
