# Data Model And Sync Flow

This document explains the SQLite schema and the lifecycle of attendance, task, and screenshot records.

## 1. Why the local database matters

The app is local-first. SQLite is not just a cache. It is the working system of record during:

- offline operation
- interrupted sync
- app restart after partial failure
- manual resync

If you change sync behavior, always preserve enough local information for a later retry.

## 2. Storage locations

### Database

Resolved in `backend/db/db.go`.

On Windows, the database is created under the user's config directory in a `wails-app` folder.

- development build: `fesapp-dev.db`
- production build: `fesapp.db`

### Screenshot files

Resolved in `backend/services/storage.go`.

- development mode: `<repo>/backend/screenshots`
- production mode: user cache directory under `FESApp/screenshots`

### Credentials

Remote tokens and keys are stored in the OS keyring, not in SQLite:

- FES API token
- Backblaze credentials

## 3. Core tables

### `users`

Purpose:

- local account registry
- cached employee profile details

Important fields:

- `id`
- `email`
- `name`
- `position`
- `birthdate`
- `phone`
- `address`
- `photo`
- `fes_id`
- `employee_id`
- `employment_id`

### `clients`

Purpose:

- local cache of remote client records

Important fields:

- `id`
- `fullname`
- `company_name`

### `tasks`

Purpose:

- local cache of remote project/task choices
- lookup for tracking and summaries

Important fields:

- `task_id`
- `title`
- `project_name`
- `client_id`

### `attendances`

Purpose:

- one login/logout work session

Important fields:

- `id`
- `user_id`
- `time_in`
- `time_out`
- `login_logout_id`
- `sync_status`
- `attempt_count`
- `last_error`
- `last_attempt_at`
- `synced_at`

### `task_records`

Purpose:

- a tracked work segment for a specific task within an attendance session

Important fields:

- `id`
- `user_id`
- `attendance_id`
- `task_id`
- `started_at`
- `ended_at`
- `duration`
- `padagan`
- `notes`
- `productivity`
- `gap`
- `sync_status`
- `attempt_count`
- `last_error`
- `last_attempt_at`
- `synced_at`

### `screenshots`

Purpose:

- metadata for captured screenshots associated with a task record

Important fields:

- `id`
- `task_record_id`
- `task_id`
- `memo`
- `notes`
- `file_path`
- `cloud_url`
- `taken_at`

## 4. Relationship model

```text
User
  -> many Attendances

User
  -> many TaskRecords

Attendance
  -> many TaskRecords

Task
  -> many TaskRecords

TaskRecord
  -> many Screenshots

Client
  -> many Tasks
```

## 5. Migrations and schema repair

The schema is managed through a migration catalog, not by naive auto-migration alone.

Important characteristics:

- canonical schemas are defined explicitly
- legacy tables can be rebuilt into the canonical form
- migration planning decides whether a backup is required
- hot query indexes are created explicitly

This is designed to recover older or partially corrupted local databases instead of assuming a perfect history.

## 6. Sync status lifecycle

The app uses explicit sync states.

| State | Meaning |
| --- | --- |
| `pending` | record exists locally and still needs remote sync |
| `in_progress` | a sync worker is currently attempting to process the record |
| `synced` | remote sync succeeded |
| `failed` | a sync attempt was made and failed |
| `blocked` | sync could not proceed because a required condition was not met |

### Common reasons for `blocked`

- the app is offline
- attendance dependency is not ready for task sync
- remote identity needed for downstream sync is missing

## 7. Attendance lifecycle

### Online login

1. user successfully logs in against the FES API
2. a local attendance is created or reused
3. `SyncService` attempts remote attendance login sync
4. remote response may return a `login_logout` identifier
5. local attendance is marked synced when successful

### Logout

1. local attendance gets `time_out`
2. sync status becomes `pending`
3. remote logout sync is attempted
4. local attendance becomes `synced` only if remote logout succeeds

### Resync guard

Manual attendance resync is only allowed within a 48-hour window, enforced in `App.SyncAttendanceByID`.

## 8. Task record lifecycle

### Start tracking

1. any lingering active task records are closed defensively
2. a new task record is created locally
3. `gap` is calculated from the previous ended record in the same attendance
4. the new record is eligible for sync
5. if online, `SyncService` tries remote task-in sync

### Stop tracking

1. the open task record is closed locally
2. `duration` is computed
3. `sync_status` is reset to `pending`
4. if online, remote task-out sync is attempted

### Why task sync may happen in multiple steps

Remote task sync can be split into:

- remote task-in
- remote task-out

The local `productivity` identifier returned from task-in is needed for task-out and for some screenshot-related remote correlation.

## 9. Attendance-before-task dependency

Task sync depends on attendance readiness.

`ensureAttendanceReadyForTaskSync(...)` does the following:

- reload the attendance from the database
- check for a remote `login_logout` or a synced attendance state
- attempt attendance sync if needed
- retry a small number of times

This prevents task records from syncing against a remote session that does not exist yet.

## 10. Screenshot lifecycle

Current code reality:

- screenshot records and plumbing exist
- actual capture is disabled by a compile-time constant
- cloud upload is also disabled by a compile-time constant

If enabled, the intended flow is:

1. `ScreenshotService` captures all active displays
2. files are stored locally as `.webp`
3. a `screenshots` row is inserted in SQLite
4. frontend receives a `screenshot` runtime event
5. `SyncService` uploads missing screenshots after task record sync

### Pending screenshot handling

The sync layer can detect screenshots where `cloud_url` is empty and attempt backlog upload later.

This is why the activity log page can show screenshot backlog status.

## 11. Summary and activity log queries

### Activity logs

Implemented in `backend/services/activity_log.go`.

It:

- resolves the Manila date range
- queries attendances for the selected date
- loads task records for each attendance
- counts pending screenshots when screenshot uploads are relevant

### Daily summaries

Implemented in `backend/services/summary.go`.

It:

- groups task records by task and `padagan`
- sums durations
- builds user-facing task names using client and task fields

## 12. Timezone model

The app is Manila-time aware in both frontend and backend.

Why that matters:

- attendance filters are date-based
- logs and summaries depend on correct local-day boundaries
- naive UTC assumptions can move records to the wrong calendar day

Relevant files:

- backend date helpers used by activity log and summary services
- `frontend/src/lib/timezone.ts`

If you change date logic, test around midnight boundaries and timezone fallback paths.

## 13. Debugging sync problems

When a user says "my time did not sync", inspect these in order:

1. `attendances.sync_status`
2. `attendances.login_logout_id`
3. `attendances.last_error`
4. `task_records.sync_status`
5. `task_records.productivity`
6. `task_records.last_error`
7. `screenshots.cloud_url` for screenshot-related issues

### Helpful signals

- `attempt_count` tells you whether the worker actually retried
- `last_attempt_at` tells you when it last tried
- `blocked` usually means preconditions were missing
- `failed` usually means a real attempt happened and errored

## 14. Diagnostic bundle

The backend can export a diagnostic bundle containing:

- app version and platform metadata
- recent attendance sync states
- recent task record sync states
- current log file if available

This is the best support artifact when debugging a production sync issue.

## 15. Data and sync cautions

### Do not clear or rewrite sync columns casually

Fields like `sync_status`, `synced_at`, `attempt_count`, and `last_error` are part of operational recovery.

### A locally "complete" record may still be remotely incomplete

A task record with `ended_at` set can still be unsynced if remote task-out failed.

### Backfill logic matters

Migration repair logic is designed to preserve existing user data. Avoid shortcuts that only work on fresh databases.

## 16. Read next

- `guide/backend.md` for the Go services implementing these rules
- `guide/developer-workflow.md` for debugging and release checklists
