# Troubleshooting Guide

This document is the practical debugging guide for real incidents.

Use it when:

- the app starts but behaves incorrectly
- sync state looks wrong
- a user's work did not appear remotely
- screenshots are missing
- the frontend looks stale
- you need a disciplined way to narrow the problem down

## 1. Start with the right mindset

In this project, a user-visible failure can come from several different layers:

- frontend page state
- Wails binding contract
- backend session state
- SQLite data
- sync queue logic
- remote FES API behavior
- local screenshot files
- Backblaze upload state

Do not guess which layer is broken. Narrow it down.

## 2. Always classify the incident first

Before touching code, decide which class the bug seems to belong to.

### Class A: startup or packaging

Examples:

- app will not launch
- DB initialization error
- update flow failure

### Class B: authentication/session

Examples:

- login fails
- offline login fails
- user state looks wrong after login

### Class C: tracking lifecycle

Examples:

- task will not start
- task will not stop
- timer UI is wrong

### Class D: sync failure

Examples:

- record stays pending
- attendance says not synced
- manual resync fails

### Class E: screenshot failure

Examples:

- no screenshot preview
- screenshot files missing
- screenshots never upload

### Class F: stale frontend state

Examples:

- backend did work but screen did not refresh
- sync finished but badge did not update

## 3. Golden debugging sequence

This sequence works well for most incidents:

1. reproduce the issue
2. identify the exact user action
3. identify the frontend method or button involved
4. find the backend `App` method called
5. inspect local DB changes
6. inspect sync-state fields
7. inspect logs
8. inspect runtime-event expectations

## 4. When the app will not start

Check these first:

1. `main.go`
2. `backend/app/app.go`
3. `backend/db/db.go`
4. file-system permissions for config/cache paths
5. whether database initialization failed

Important startup facts:

- the app quits if DB initialization fails
- startup also sets up logging and the tray

## 5. When login fails

### Online login debugging

Inspect:

1. `frontend/src/pages/Auth/Login.tsx`
2. `App.LoginUserOnline`
3. `AuthService.Login`
4. `AuthService.AttemptLogin`
5. `FesClientService.Request`

Questions to answer:

- did the frontend call the binding?
- did the remote FES API return an error?
- was a token saved?
- was a local user created?
- was attendance created locally?

### Offline login debugging

Inspect:

1. `frontend/src/pages/Auth/OfflineLogin.tsx`
2. `App.LoginUser`
3. local user table contents

Questions to answer:

- does the local user already exist?
- was local attendance created?
- did backend connectivity switch to offline?

## 6. When a task will not start

Inspect:

1. `frontend/src/pages/Main/Task.tsx`
2. `App.StartTracking`
3. `TaskService.StartTracking`
4. `TaskService.StartTaskRecord`
5. related repository methods

Questions to answer:

- is there an active attendance?
- did a new local task record get created?
- did defensive cleanup of old records fail?
- was the task considered a Break task?
- did the screenshot worker start if expected?

## 7. When a task will not stop

Inspect:

1. `Task.tsx` stop action
2. `App.StopTracking`
3. `TaskService.StopTracking`
4. local `task_records` row state after stop

Questions to answer:

- did `ended_at` get written?
- did `duration` get updated?
- was `sync_status` reset to `pending`?
- was sync attempted if online?

## 8. When a record says "Not synced"

This is one of the most common and most important incidents.

Inspect in this order:

1. attendance row
2. attendance `login_logout_id`
3. attendance `sync_status`
4. attendance `last_error`
5. task record `sync_status`
6. task record `productivity`
7. task record `last_error`

### Common interpretations

- attendance missing `login_logout_id`: task sync may be blocked upstream
- task record missing `productivity`: task-in may never have completed remotely
- `blocked`: prerequisites missing
- `failed`: actual request error

## 9. When manual resync fails

Inspect:

1. which resync button or action was used
2. whether the record belongs to the current authenticated user
3. whether attendance is outside the 48-hour resync window
4. backend errors returned from `App.SyncAttendanceByID` or `App.SyncUnsyncedTaskRecord`

Important rule:

attendance resync is intentionally time-limited.

## 10. When screenshots do not appear locally

Inspect:

1. `frontend/src/config/release.ts`
2. `backend/service_classes/screenshot.service.go`
3. `TaskService.StartTracking`
4. the current task type
5. local screenshot storage directory
6. `screenshots` table rows

Questions to answer:

- are screenshots enabled in the frontend?
- are screenshots enabled in the backend?
- is the task a Break task?
- did the screenshot worker start?
- were image files written?
- was the screenshot row inserted?

## 11. When screenshots exist locally but do not upload

Inspect:

1. `screenshots.cloud_url`
2. `task_records.productivity`
3. backend online state
4. Backblaze credentials in keyring
5. `backend/services/backblaze.go`
6. `SyncService.drainPendingScreenshotsNow(...)`

Questions to answer:

- does the task record have a remote productivity ID?
- is Backblaze enabled?
- is the app online?
- do the local files still exist?
- did upload return errors?

## 12. When the frontend looks stale

This often happens when backend work succeeded but the page did not refresh.

Inspect:

1. whether the backend emitted the expected runtime event
2. whether the page subscribed with `EventsOn(...)`
3. whether the event handler refreshed local state
4. whether the Zustand store was updated

Common event names to check:

- `record-synced`
- `attendance-synced`
- `employee_details_fetched`
- `tasks-sync-warning`
- `screenshot`

## 13. Useful DB inspection checklist

When using a SQLite browser or direct SQL, these are useful checks:

### Attendance health

- latest row for the user
- `time_in`
- `time_out`
- `login_logout_id`
- `sync_status`
- `last_error`

### Task-record health

- latest row for the task
- `started_at`
- `ended_at`
- `productivity`
- `sync_status`
- `last_error`

### Screenshot health

- latest screenshot row
- `file_path`
- `cloud_url`
- whether the referenced local files exist

## 14. Useful file-system inspection checklist

Check:

- screenshot storage root
- whether per-user folders exist
- whether screenshot image files exist
- whether logs exist

Remember:

- the DB may say a screenshot exists even if the local file was later deleted manually

## 15. Useful code-reading shortcuts by symptom

### "Task list is wrong"

Read:

1. `frontend/src/pages/Main/Home.tsx`
2. `TaskService.GetTasks`
3. `FesClientService.SyncClientsAndProjects`

### "Summary totals are wrong"

Read:

1. `frontend/src/lib/utils.ts`
2. `backend/services/summary.go`
3. date/timezone helpers

### "Activity logs are wrong"

Read:

1. `frontend/src/pages/Main/ActivityLogs.tsx`
2. `backend/services/activity_log.go`
3. `guide/data-and-sync.md`

## 16. When logs matter most

Logs are especially useful when:

- the DB says sync attempts happened
- the frontend only shows a vague error
- you need to distinguish remote API failure from local state failure

Watch for:

- operation names
- correlation IDs
- sync start/fail/success lines
- update-check decisions

## 17. Fast decision tree

Use this quick map when you are under time pressure:

### Local record missing entirely

Likely problem area:

- frontend did not call backend
- backend refused the action before persistence
- repository write failed

### Local record exists but remote state missing

Likely problem area:

- sync queue
- remote API failure
- attendance dependency
- offline state

### Local record synced but UI wrong

Likely problem area:

- runtime event not emitted
- runtime event not handled
- stale Zustand state

### Screenshot row exists but file missing

Likely problem area:

- file-system cleanup or path issue
- manual deletion

### Screenshot file exists but cloud URL empty

Likely problem area:

- Backblaze connectivity
- missing productivity ID
- offline state

## 18. Best final question before fixing

Before you write the fix, ask:

"Is the real bug in record creation, state transition, remote sync, event propagation, or UI refresh?"

That question prevents a lot of false fixes.

## Read next

If troubleshooting showed you the incident belongs to data lifecycle or sync ordering, go deeper into [`guide/data-and-sync.md`](./data-and-sync.md).
