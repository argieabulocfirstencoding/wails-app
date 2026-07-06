# Data And Sync

This document explains the local database, the record lifecycle, and the synchronization model.

If a production bug sounds like:

- "my login did not sync"
- "my task stopped but did not appear remotely"
- "screenshots are missing"
- "the app says not synced"

then this is the document you want first.

## 1. Why local data matters so much

The local SQLite database is not just a cache for convenience.

It is the app's durable operational memory during:

- normal work
- offline work
- partially failed sync
- app restart after interruption
- manual recovery and resync

That means local records must be good enough to tell the truth even when the remote system is behind.

## 2. Where data is stored

### SQLite database

Resolved in:

- `backend/db/db.go`

The database lives under the user's config directory in a `wails-app` folder.

Build-specific filenames:

- development: `fesapp-dev.db`
- production: `fesapp.db`

### Screenshot image files

Resolved in:

- `backend/services/storage.go`

Storage behavior:

- development: `<repo>/backend/screenshots`
- production: user cache directory under `FESApp/screenshots`

### Secrets and tokens

Stored in the OS keyring, not in SQLite.

Examples:

- `fes_api_token`
- `APP_ID`
- `APP_KEY`

## 3. Core tables

The following tables are the ones a takeover developer should understand first.

### `users`

Purpose:

- stores known local accounts
- caches employee profile details returned from the remote system

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

- local cache of remote client entities

Important fields:

- `id`
- `fullname`
- `company_name`

### `tasks`

Purpose:

- local cache of remote task/project choices
- lookup data used during tracking and summaries

Important fields:

- `task_id`
- `title`
- `project_name`
- `client_id`

### `attendances`

Purpose:

- represents one work-session envelope, usually login -> logout

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
- `client_time`
- `total_gap`

### `task_records`

Purpose:

- represents one tracked segment of work on a specific task inside an attendance session

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

- stores metadata for captured screenshot batches associated with a task record

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

In simple terms, the relationships are:

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

### Why this matters

If a task record looks broken, you often need to inspect:

- its attendance
- its task
- its screenshots

not just the row itself.

## 5. Why schema work is handled carefully

The app already has migration and repair infrastructure because local data is valuable and older installations may drift.

That means schema changes should preserve these principles:

- be explicit
- be testable
- be repair-friendly
- preserve existing user data

## 6. The sync-state machine

Both attendances and task records use explicit sync-state columns.

### Main states

| State | Meaning in plain language |
| --- | --- |
| `pending` | the record exists locally and still needs to be synced |
| `in_progress` | the app is actively attempting sync work right now |
| `synced` | the remote operation completed successfully |
| `failed` | a real sync attempt happened and returned an error |
| `blocked` | sync could not proceed because a required condition was missing |

### Common reasons for `blocked`

- the app is offline
- the task depends on attendance data that is not ready yet
- the record is missing a remote identifier required for the next stage

### Why `failed` and `blocked` are different

- `failed` usually means "we tried and got an error"
- `blocked` usually means "we intentionally did not proceed because prerequisites were missing"

That difference is very useful during debugging.

## 7. Sync metadata columns and why they matter

Important columns include:

- `sync_status`
- `attempt_count`
- `last_attempt_at`
- `last_error`
- `synced_at`

These are not optional decoration. They help answer:

- did the app even try?
- how many times did it try?
- what was the last failure?
- when did it last try?
- when did it finally succeed?

Do not casually wipe these values.

## 8. Attendance lifecycle

An attendance record is the container around a session of work.

### Online login path

1. user logs in through the remote FES API
2. a local attendance is created
3. sync service tries to perform remote attendance login
4. remote response may return `login_logout`
5. local attendance is marked `synced`

### Logout path

1. local attendance gets a `time_out`
2. sync status becomes relevant again because logout must be synced too
3. sync service attempts remote logout
4. on success, the attendance is marked `synced`

### Important field: `login_logout_id`

This is one of the most important values in the entire app. It is the remote attendance/session identity used by downstream task syncing.

## 9. Task record lifecycle

Task records represent actual units of tracked work.

### Start tracking

When a task starts:

1. defensive cleanup may close old active records
2. a new task record is created locally
3. `gap` is computed from the previous record in the same attendance
4. sync service may attempt remote task-in if possible
5. screenshot capture may start

### Stop tracking

When a task stops:

1. the local open record is closed
2. `ended_at` is written
3. `duration` is calculated
4. sync state is reset to `pending`
5. remote task-out is attempted if online

### Why task sync can be multi-stage

Remote task syncing is not always a single request.

There are conceptually two phases:

- task-in
- task-out

The app often needs a remote `productivity` identifier from task-in before it can correctly complete task-out and related screenshot upload behavior.

## 10. Attendance-before-task dependency

This is one of the most important rules in the app.

Task sync depends on attendance readiness.

In practice, `SyncService.ensureAttendanceReadyForTaskSync(...)` checks whether:

- the attendance exists
- it already has a usable `login_logout_id`
- or it is otherwise already considered synced enough

If not, the sync service retries attendance sync first.

### Practical meaning

If a task record refuses to sync, do not inspect only the task record. Inspect its attendance.

## 11. Screenshot lifecycle

Screenshots have both local and remote lifecycle stages.

### Local capture phase

1. screenshot service detects active displays
2. each display is captured
3. each image is encoded as WebP
4. files are written to disk
5. a `screenshots` row is written to SQLite
6. a frontend runtime event is emitted

### Upload phase

1. sync service checks whether the task record has a remote `productivity` ID
2. sync service checks whether Backblaze is enabled and connected
3. pending screenshot files are uploaded
4. `cloud_url` is updated

### Important detail: `file_path`

The screenshot row stores local path information, often as JSON-encoded paths. That means screenshot debugging sometimes requires inspecting both:

- the database row
- the real files on disk

## 12. What "pending screenshot backlog" really means

A screenshot can exist locally and still be waiting for cloud upload.

That usually means:

- local capture succeeded
- the `screenshots` row exists
- `cloud_url` is still empty

This is why the activity log and sync logic treat screenshot uploading as a later reconciliation step.

## 13. Sync queue behavior

The sync service does not fire requests blindly. It manages a queue with retries and deduplication.

### Queue characteristics

- jobs have keys
- duplicate pending jobs may be skipped
- retryable failures are retried with backoff
- blocked failures are not retried forever as if they were network flukes

### Why this is good

Without queueing and deduplication, a user rapidly clicking around could trigger overlapping or contradictory sync attempts.

## 14. What "source of truth" means here

Different pieces of data have different owners.

### Practical ownership model

- React store owns short-lived UI state
- `App` owns current runtime session wiring
- SQLite owns durable local operational state
- FES API owns authoritative remote records after successful sync
- Backblaze owns remote screenshot objects after upload

During debugging, first decide which layer should be trusted for the problem you are investigating.

## 15. Helpful SQL checks during debugging

If you inspect the SQLite database directly, these kinds of queries are useful:

```sql
SELECT id, user_id, time_in, time_out, login_logout_id, sync_status, attempt_count, last_error
FROM attendances
ORDER BY id DESC
LIMIT 20;
```

```sql
SELECT id, task_id, attendance_id, started_at, ended_at, productivity, sync_status, attempt_count, last_error
FROM task_records
ORDER BY id DESC
LIMIT 20;
```

```sql
SELECT id, task_record_id, task_id, file_path, cloud_url, taken_at
FROM screenshots
ORDER BY id DESC
LIMIT 20;
```

### What to look for

- records stuck in `blocked`
- records stuck in `failed`
- empty `login_logout_id` on attendance
- empty `productivity` on task records that should have remote identity
- empty `cloud_url` when screenshot upload should have happened

## 16. Timezone model

The app cares about Manila-time boundaries for summaries and logs.

Why this matters:

- a task near midnight can land on the wrong day if timezone logic is careless
- attendance summaries depend on date boundaries, not just absolute timestamps

If you touch date filtering, test around midnight and timezone conversion behavior.

## 17. Manual resync guard

Attendance resync is intentionally time-limited.

`App.SyncAttendanceByID(...)` currently enforces a 48-hour resync window.

That means older login/logout records may intentionally reject manual resync even if they look tempting to repair.

## 18. How to debug "not synced" reports

When a user says "my time did not sync", inspect in this order:

1. the attendance row
2. the attendance `login_logout_id`
3. the attendance `sync_status`
4. the attendance `last_error`
5. the task record `sync_status`
6. the task record `productivity`
7. the task record `last_error`
8. related screenshot `cloud_url` values if screenshots are involved

### Good interpretation habits

- `attempt_count = 0` often means the worker never really got a chance
- `blocked` often means a prerequisite was missing
- `failed` usually means a real request errored
- a closed local record can still be remotely unsynced

## 19. Things you should not change casually

- sync-state semantics
- `login_logout_id` handling
- `productivity` handling
- screenshot file-path format
- migration repair strategy
- index strategy for hot query paths

## Read next

- [`guide/backend.md`](./backend.md) for the code that implements these rules
- [`guide/troubleshooting.md`](./troubleshooting.md) for step-by-step incident debugging
