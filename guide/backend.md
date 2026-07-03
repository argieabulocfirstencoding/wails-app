# Backend Deep Dive

This document explains the Go side of the application.

## 1. Backend folder layout

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

## 2. Process entry point

Files:

- `main.go`
- `backend/app/app.go`

`main.go` performs the Wails bootstrap:

- loads `.env`
- checks `DEVELOPMENT`
- creates `app.NewApp()`
- binds that `App` instance into Wails
- configures window size, single-instance lock, asset server, and shutdown hooks

`backend/app.App` is therefore the primary boundary object between frontend and backend.

## 3. The `App` facade

`backend/app/app.go` is the most important backend file to understand first.

It has three jobs:

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

## 4. Database bootstrap

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

### Migration strategy

This codebase no longer relies on a simple `AutoMigrate` path.

It uses:

- a migration catalog
- schema planning
- canonical table rebuilds when required
- pre-migration backups when destructive repair may occur
- explicit index creation

That is a serious piece of infrastructure. Any schema change should preserve this philosophy.

## 5. Data model

The most important GORM models are:

- `User`
- `Attendance`
- `Task`
- `TaskRecord`
- `Screenshot`
- `Client`

These models live under `backend/db/models/`.

There are also exported frontend-facing shapes such as:

- `ExpTask`
- `ExpTaskRecord`
- `ExpAttendance`
- `ActivityLogRecord`
- `TaskRecordWithScreenshots`

These shapes matter because Wails exposes them to TypeScript via generated bindings.

## 6. Repository layer

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

## 7. Current service architecture

### `service_classes`

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

### `services`

This package is mixed and should be read carefully before modification. It contains:

- activity log and summary queries
- storage helpers
- Backblaze integration
- old login/attendance helpers still used in some paths
- other utility-style domain functions

Some files are still important and current. Others overlap with the newer layered structure.

## 8. Authentication flow

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

## 9. Task tracking flow

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

## 10. Sync engine

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

## 11. Screenshot subsystem

Files:

- `backend/service_classes/screenshot.service.go`
- `backend/services/storage.go`
- `backend/services/backblaze.go`

### Current behavior

- screenshot capture is compiled behind `const screenshotsEnabled = false`
- cloud upload is compiled behind `const backblazeEnabled = false`
- local storage helpers still exist
- screenshot records and DB schema still exist

### What the code is prepared to do

If enabled, the screenshot service would:

- capture all active displays
- save local `.webp` files
- store screenshot metadata in SQLite
- emit a frontend runtime event
- ask `SyncService` to reconcile or upload screenshots after task sync

## 12. External integrations

### FES API

Handled mainly by `FesClientService`.

Key responsibilities:

- login
- employee detail fetch
- task/client/project sync
- login attendance sync
- logout attendance sync
- task in / task out sync
- Backblaze credential retrieval
- magic-link open for the FES web app

### Backblaze B2

Handled by `BackblazeSync`.

Current reality:

- upload code exists
- keyring-based credential flow exists
- feature is disabled by constant

### Upwork helper flow

The app includes an automation path that retrieves credentials and helps open the relevant environment for Upwork-related tasks. This is desktop-specific and mixes remote data, OS-level behavior, and clipboard/keyboard automation.

## 13. Updates and installer flow

`App.CheckForUpdates` and `App.DownloadAndInstall` implement a GitHub-release-driven update process.

Important characteristics:

- checks only trusted GitHub hosts
- selects installer assets from release metadata
- validates content type
- optionally verifies checksums if present
- launches the installer and then quits the app

If you touch release or update behavior, inspect both:

- `backend/app/app.go`
- `wails.json`

## 14. Diagnostics and observability

The backend includes:

- structured-ish logging through `utils.LoggerService`
- correlation ids
- panic recovery helpers
- a diagnostic bundle export path

The diagnostic bundle currently packages:

- manifest metadata
- recent attendance sync state
- recent task record sync state
- current log file, when available

This is useful for support and post-failure analysis.

## 15. Error design

The backend uses two notable error patterns:

### `StableError`

Defined in `backend/types/error.type.go`.

Purpose:

- return safe, user-facing error messages across the Wails boundary
- avoid leaking internal details to the frontend

### `OperationError`

Purpose:

- wrap internal errors with operation name, record id, and user id for diagnostics

`App.stableClientError(...)` is the main adapter that converts internal errors into frontend-safe ones.

## 16. Backend tests

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

When changing sync, migrations, or task tracking, add or update tests in the nearest package rather than relying on manual verification only.

## 17. Backend caveats and maintenance advice

### Do not assume every "service" is equal

Read the call graph before moving code between `services` and `service_classes`.

### Protect local data first

If remote sync fails, the local DB should still tell the truth about what happened.

### Be careful with shared in-memory pointers

The app wires pointers like `Attendance`, `TaskRecord`, and `WorkingTaskID` across services. Changes here can create subtle state bugs.

### Review shutdown behavior when altering task lifecycles

Shutdown intentionally tries to leave active work in a retryable local state.

## 18. Read next

- `guide/data-and-sync.md` for schema and lifecycle details
- `guide/frontend.md` to understand how UI actions hit these backend methods
