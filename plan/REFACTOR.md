# Refactor And Sync Defect Plan

Generated on 2026-05-28 after inspecting the Wails v2 app layout, backend services, generated bindings, frontend state/events, existing tests, and build results.

## Scope

This plan targets refactoring and bug fixes for attendance, task tracking, screenshots, syncing, lifecycle cleanup, and testability. No implementation changes are included in this document.

Primary areas inspected:

- `main.go`
- `wails.json`
- `go.mod`
- `backend/app/app.go`
- `backend/db/db.go`
- `backend/repository/*.go`
- `backend/services/*.go`
- `backend/service_classes/*.go`
- `backend/tests/*.go`
- `frontend/src/App.tsx`
- `frontend/src/pages/Main/Home.tsx`
- `frontend/src/pages/Main/Task.tsx`
- `frontend/src/pages/Auth/Login.tsx`
- `frontend/src/pages/Auth/Logout.tsx`
- `frontend/src/store/AuthContext.ts`
- `frontend/wailsjs/go/app/App.d.ts`

## Current Project Shape

- Wails version: `github.com/wailsapp/wails/v2 v2.12.0`.
- Frontend: React 18, Vite, TypeScript, Zustand, React Router, Tailwind/Radix.
- Persistence: SQLite through GORM, initialized globally as `db.DB`.
- Runtime integrations: Wails runtime events, systray, screenshots, Backblaze B2, FES API, older Firebase sync code.
- Backend structure currently mixes:
  - Wails binding and lifecycle code in `backend/app/app.go`.
  - Domain/application behavior in both `backend/services` and `backend/service_classes`.
  - Direct global database usage in repositories, services, and bindings.
  - Two parallel sync implementations: older Firebase sync in `backend/services/sync.go` and newer FES sync in `backend/service_classes/sync.service.go`.

## Verification Snapshot

Commands run:

- `go test ./...`
  - Fails in `fesapp/backend/tests`.
  - `TestOfflineLoginFlow` expects a `uint` attendance ID but `services.LoginUser` now returns `*models.Attendance`.
  - `TestFesOnlineLogin` performs a real login request and fails with invalid credentials. This should not be a normal unit test.
- `npm run build` from `frontend`
  - Passes TypeScript and Vite build.
  - Vite warns that `assets/index-*.js` is larger than 500 kB after minification.

These results should be treated as baseline failures before refactoring.

## Highest Risk Findings

### 1. Task start logic can panic or silently corrupt tracking state

Files:

- `backend/service_classes/task.service.go`
- `backend/repository/task.repo.go`

Findings:

- `TaskRepository.CheckTaskRecord` returns `(nil, nil)` when no record exists.
- `TaskService.StartTaskRecord` treats `err == nil` as an existing record path, then logs `nil`; the `else` branch can call `updateRecord(taskRecord)` with a nil record depending on behavior.
- `StartTaskRecord` creates a record and immediately dereferences `record` before checking `err`.
- `StartTracking` sets screenshot `TaskRecordID = &record.ID` even if `record` is nil.
- `StopTracking` reads `(*t.Attendance).UserID` before checking whether `t.Attendance` is nil.

Planned fix:

- Make repository methods return `gorm.ErrRecordNotFound` or a typed `ErrNotFound`; do not convert not-found into `(nil, nil)` unless the method name explicitly uses optional semantics.
- Add a single application-level `StartTracking(ctx, request)` use case that:
  - Validates attendance exists.
  - Stops or closes any prior active records in one transaction.
  - Creates the new task record.
  - Starts screenshot capture only after the DB record is committed.
  - Returns a stable DTO to the frontend.
- Add tests for no active record, existing active record, DB create failure, and break/training tasks.

### 2. Sync state is not idempotent and can mark records pending incorrectly

Files:

- `backend/service_classes/sync.service.go`
- `backend/services/sync.go`
- `backend/service_classes/task.service.go`
- `backend/services/screenshot.go`

Findings:

- There are two sync services with overlapping names and behavior.
- `SyncTaskRecord` in the newer service always sets `sync_status = "pending"` before attempting remote sync.
- `SyncTaskRecord` decides between `task_in` and `task_out` only from `EndedAt`, but does not persist remote IDs consistently.
- `SyncAttendance` emits `attendance-synced` false when offline, but returns nil, which makes caller behavior ambiguous.
- `SyncActiveTaskRecord` condition uses `if !config.OnlineMode && (*s.Attendance).LoginLogoutID == nil`, which only fails when both are true; the likely intended check is offline OR missing login ID.
- `SyncAllAttendance`, `SyncUnsyncedRecords`, and `SyncUser` are stubs in the newer service.
- Older Firebase sync has queue logic, retries, token refresh, and screenshot upload coupling, but is partially deprecated and still reachable from `App`.

Planned fix:

- Choose one sync provider abstraction and make implementation explicit:
  - `AttendanceSyncer`
  - `TaskRecordSyncer`
  - `ScreenshotUploader`
  - `RemoteTaskCatalog`
- Remove or quarantine deprecated Firebase sync from active tracking flows after tests cover current FES behavior.
- Add explicit sync statuses: `local`, `pending`, `syncing`, `synced`, `failed`.
- Store sync attempt metadata: `last_sync_error`, `sync_attempts`, `remote_id`, `synced_at`.
- Make every sync operation idempotent:
  - `task_in` should be skipped or updated if remote productivity ID already exists.
  - `task_out` should require a known remote record ID/productivity ID.
  - Attendance login/logout should be separate operations with persisted remote login/logout ID.
- Emit events from a dedicated event bridge, not from repository/service internals.

### 3. App lifecycle and goroutine shutdown are incomplete

Files:

- `backend/app/app.go`
- `backend/services/screenshot.go`
- `backend/services/backblaze.go`

Findings:

- `NewApp` calls `TaskService.Connect(app.ctx)` and `syncService.Connect(app.ctx)` before Wails startup assigns `a.ctx`; services receive nil context.
- Systray is started in a goroutine and the click listener loops forever; shutdown does not call a systray quit path.
- Screenshot service uses a reusable `stopChan`; after one stop/start cycle, sends can be missed or block depending on state.
- Screenshot and upload goroutines are not tied to cancellable operation contexts.
- Backblaze reconnect uses `context.Background()` and can outlive app shutdown.
- `DownloadAndInstall` calls `os.Exit(0)` directly after starting installer, bypassing normal Wails shutdown cleanup.

Planned fix:

- Move all runtime-dependent service `Connect(ctx)` calls into `Startup`.
- Add an app root context with `context.WithCancel` owned by the Wails lifecycle.
- Make screenshot capture a managed worker with:
  - `Start(ctx, taskRecordID, taskID, folderName)`
  - `Stop(ctx) error`
  - guarded state under a mutex.
- Replace `context.Background()` in background sync/upload with app or job contexts.
- Add shutdown ordering:
  - Stop screenshot worker.
  - Stop active task.
  - Flush best-effort sync queue with timeout.
  - Stop systray.
  - Close DB connection.

### 4. Wails binding methods are too stateful and too broad

Files:

- `backend/app/app.go`
- `frontend/wailsjs/go/app/App.d.ts`

Findings:

- `App` exposes many methods directly to the frontend and also owns mutable session state.
- Binding methods call repositories, services, runtime dialogs, network APIs, filesystem operations, and updater code directly.
- Several methods can dereference nil state when called out of sequence:
  - `FetchUserDetails`
  - `GetActiveTaskRecord`
  - `SyncUnsyncedAttendance`
  - `GetCurrentAttendance`
  - `LogoutUser`
- `BeforeClose(ctx context.Context)` appears in generated bindings, but Wails lifecycle callbacks are not meant to be invoked from frontend with a real Go context.

Planned fix:

- Split Wails-bound API into thin facades:
  - `SessionBinding`
  - `TaskBinding`
  - `SyncBinding`
  - `SystemBinding`
  - or keep one `App` type but delegate immediately to application services.
- Use typed request/response DTOs for non-trivial methods:
  - `StartTrackingRequest`
  - `StopTrackingRequest`
  - `SyncTaskRecordRequest`
  - `LoginRequest`
- Return stable UI-facing errors with code/message/details.
- Add input and session validation at the binding boundary.

### 5. Frontend state can drift from backend tracking state

Files:

- `frontend/src/pages/Main/Home.tsx`
- `frontend/src/pages/Main/Task.tsx`
- `frontend/src/App.tsx`
- `frontend/src/store/AuthContext.ts`

Findings:

- Frontend owns `taskID`, timers, `isStarted`, sync flags, and local storage values independently from backend DB state.
- `Task.tsx` starts local timer with `Date.now()` before backend confirms the persisted record start time.
- `Home.tsx` only fetches active record once due to `fetchedActiveRecord`; state can become stale after sync, logout, task switch, or second instance restore.
- `App.tsx` listens for `app_shutdown` and calls `StopTracking`, while backend `Shutdown` also calls `StopTracking`; duplicate stop is possible.
- `Task.tsx` sets `isSyncing` true on screenshot event and only clears it on `record-synced`, which can remain stuck if upload/sync is skipped or failed before event emission.
- Login cancel only hides loading; the backend login call is not cancellable.

Planned fix:

- Add a single frontend API client layer around Wails bindings.
- Make backend return authoritative tracking session DTOs:
  - active task record ID
  - task ID
  - started at
  - ended at
  - sync status
  - screenshot status
- Replace local optimistic timer starts with backend-confirmed start times.
- Add event names and payload contracts:
  - `tracking:started`
  - `tracking:stopped`
  - `tracking:error`
  - `sync:attendance`
  - `sync:task-record`
  - `screenshot:captured`
  - `screenshot:upload`
- Centralize event subscriptions in hooks with cleanup.
- Remove frontend shutdown stop call once backend shutdown is authoritative.

### 6. Persistence and transactions need stronger boundaries

Files:

- `backend/db/db.go`
- `backend/repository/*.go`
- `backend/service_classes/*.go`
- `backend/services/*.go`

Findings:

- `db.DB` is global and used across all layers, including tests.
- Multi-step flows are not transactional:
  - stop all active records + create new task record
  - stop task + sync task out
  - login attendance + sync attendance + fetch user details
  - screenshot DB insert + remote upload + cloud URL update
- DB initialization uses `log.Fatalf`, which exits the whole app instead of returning a user-facing startup error.
- AutoMigrate is used as production schema management, but there is no explicit migration strategy.

Planned fix:

- Introduce repository constructors that accept `*gorm.DB`.
- Add transaction helpers for task switching and logout.
- Return DB initialization errors to Wails startup instead of fatal exit.
- Add SQLite pragmas in DB setup and expose a small health check.
- Decide whether to keep AutoMigrate short-term or add versioned migrations before release.

## Refactor Phases

### Phase 0: Baseline and safety net

Goal: make defects measurable before changing behavior.

Tasks:

- Record current failing `go test ./...` output.
- Convert real network tests into integration tests behind build tags or environment flags.
- Fix broken unit test expectations to match current function signatures, or change signatures intentionally in Phase 1.
- Add a small fake FES client for sync tests.
- Add test fixtures for SQLite in memory/temp DB.
- Add a minimal backend test helper that wires repositories/services without Wails runtime.

Acceptance:

- `go test ./...` can run locally without real credentials or real FES API calls.
- Known behavior gaps are represented by skipped or failing tests with clear names.

### Phase 1: Stabilize tracking and attendance correctness

Goal: eliminate panic and data-loss risks in start/stop/logout.

Tasks:

- Replace optional `(nil, nil)` repository behavior with typed errors.
- Add `TrackingService` or refactor `TaskService` methods to use explicit request/response objects.
- Make start/stop idempotent:
  - Starting current task twice returns current session.
  - Starting another task closes prior active record exactly once.
  - Stopping an already stopped task returns a no-op result.
- Wrap task switch in a DB transaction.
- Fix nil pointer order in stop/logout/current attendance flows.
- Make shutdown stop the current task once from backend only.

Acceptance:

- Unit tests cover active session recovery, switching task, break task, logout with active task, and repeated stop.
- Manual Wails smoke test confirms start, screenshot, stop, logout, app close.

### Phase 2: Consolidate sync architecture

Goal: one coherent sync path with retryable, inspectable state.

Tasks:

- Define sync ports/interfaces for attendance, task record, and screenshot upload.
- Keep the active FES/Bubble sync implementation in one package and mark Firebase sync as legacy or remove from active binding paths.
- Add a sync queue with bounded workers, backoff, and app-context cancellation.
- Persist failed sync attempts and errors.
- Split attendance login and logout sync.
- Split task in and task out sync.
- Add a retry operation for unsynced attendance and records.
- Make event payloads include ID, entity type, status, and error code.

Acceptance:

- Offline tracking creates local records with pending/local status.
- Reconnect sync uploads attendance, task in/out, and screenshots in order.
- Failed remote call leaves local record intact with failed/pending status.
- Re-running sync does not duplicate remote task in/task out records.

### Phase 3: Screenshot worker hardening

Goal: make screenshot capture lifecycle safe and testable.

Tasks:

- Move active screenshot service to a single implementation. The stub in `backend/service_classes/screenshot.service.go` and active implementation in `backend/services/screenshot.go` should not coexist as parallel concepts.
- Guard `Running`, `TaskRecordID`, `TaskID`, and stop state with a mutex.
- Create a fresh cancellable context per tracking session.
- Avoid panics on path creation; return errors.
- Store screenshots under a user data directory instead of a relative `backend/screenshots` path.
- Emit separate capture and upload statuses.
- Add cleanup and storage-size policies.

Acceptance:

- Start/stop/start cycles do not leak screenshot goroutines.
- Screenshot errors do not crash the app.
- Screenshots associate with the correct task record after rapid task switching.

### Phase 4: Frontend API and event consistency

Goal: align UI state with backend authoritative state.

Tasks:

- Add `frontend/src/api/` wrappers around Wails generated bindings.
- Add typed event subscriptions in `frontend/src/hooks/`.
- Replace page-level direct Wails imports gradually.
- Store only UI state in Zustand; store authoritative session data from backend DTOs.
- Replace ad hoc sync booleans with entity-specific statuses.
- Add cancel semantics to login UI only when backend supports cancellable login, otherwise relabel as "Hide" or remove cancel.

Acceptance:

- Refresh/home/task pages show the same active task after navigation, app restore, and task switch.
- Sync indicators clear on success, failure, or offline no-op.
- No duplicate shutdown stop call from frontend.

### Phase 5: Packaging, updater, and startup behavior

Goal: reduce startup failures and unsafe exits.

Tasks:

- Move updater check out of immediate frontend mount or debounce it after session initialization.
- Validate release asset selection by OS/arch and installer naming.
- Avoid direct `os.Exit(0)` until shutdown cleanup has completed.
- Move secrets and credentials handling behind a config/credential service.
- Review `wails.json` version drift: README says 2.1.0, `wails.json` says 2.1.0, `CurrentVersion` says 2.2.3.

Acceptance:

- Updater failure cannot block login/tracking.
- App version has one source of truth.
- Installer launch path has an observable failure mode.

## Testing Plan

### Backend unit tests

Add tests for:

- Attendance:
  - create new attendance when none exists
  - reuse open attendance within cutoff
  - create new attendance after timeout/cutoff
  - logout idempotency
- Tracking:
  - start task with no active record
  - start task while same task active
  - start task while another task active
  - stop active task
  - stop missing task
  - break/training task without screenshots
  - padagan flag persistence
- Sync:
  - offline returns retryable status
  - attendance login payload
  - attendance logout payload
  - task in payload
  - task out payload
  - remote failure leaves local pending/failed
  - duplicate retry does not duplicate remote IDs
- Screenshot:
  - start requires task record and task ID
  - stop cancels worker
  - capture DB insert uses correct task record
  - upload failure records error without panic

### Repository tests

Use temp SQLite DBs, not the global app DB.

Add tests for:

- active record query by user and attendance
- last record ordering
- stop all active records
- bulk task/client upsert
- transaction rollback during task switch

### Binding tests

Instantiate binding structs with fake services.

Add tests for:

- missing session returns stable UI error
- invalid task ID returns validation error
- sync methods reject nil attendance safely
- logout with active task calls stop once

### Frontend tests

Add Vitest or equivalent before large UI refactors.

Add tests for:

- Wails API wrapper maps errors into UI-safe results.
- `useTrackingEvents` subscribes and unsubscribes.
- Home sync status updates from event payloads.
- Task page timer uses backend start time.
- Logout clears store and local storage after backend logout settles.

### Integration tests

Use build tags or environment variables for real remote services.

Add:

- FES API contract tests using known sandbox credentials only when `FES_INTEGRATION=1`.
- Backblaze upload test only when `B2_INTEGRATION=1`.
- Wails manual smoke checklist for local dev and packaged app.

Manual smoke checklist:

- Fresh install launches and initializes DB.
- Online login succeeds.
- Offline login succeeds with existing local user.
- Start task, capture screenshot, stop task.
- Start break task and verify no screenshot worker starts.
- Force offline, create records, reconnect, sync.
- Logout with active task closes attendance and task.
- Close app with active task and verify single stop.
- Launch second instance and verify first window restores.
- Update check failure does not block app.

## Proposed Target Architecture

Short-term package direction without a full rewrite:

```text
backend/
  app/                  Wails lifecycle and binding facade
  application/
    session/            login/logout/current session use cases
    tracking/           start/stop/current task use cases
    sync/               retry/sync orchestration
  domain/
    attendance.go
    task_record.go
    sync_status.go
  infrastructure/
    sqlite/             GORM repositories
    fesapi/             FES API client
    screenshots/        capture and storage worker
    b2/                 Backblaze uploader
  desktop/
    events/             Wails event bridge
```

This can be introduced incrementally by moving one workflow at a time. Do not attempt a broad directory move before the tracking/sync tests exist.

## Priority Order

1. Make tests runnable without real credentials.
2. Fix task start/stop nil and not-found semantics.
3. Move runtime `Connect(ctx)` calls out of `NewApp`.
4. Consolidate active sync implementation and status model.
5. Harden screenshot worker lifecycle.
6. Centralize frontend Wails API calls and events.
7. Clean up updater/version/config behavior.

## Known Non-Goals For First Refactor Pass

- Redesigning the UI.
- Replacing SQLite/GORM.
- Replacing Wails.
- Rewriting all services into clean architecture in one pass.
- Changing remote API contracts unless required to fix duplicate or missing sync behavior.

