**Clarified Goal**

Refactor the screenshot-to-Backblaze workflow so Backblaze folder naming is driven by `TaskRecord.Productivity`. If a task record does not have `Productivity` yet, screenshots should stay local, remain linked to the task record in the `screenshots` table, and be retried later. Every later sync attempt for that task record should check whether `Productivity` now exists and, if it does, upload all pending screenshots for that task record through a clean queued path.

Also expose pending screenshot upload counts in frontend Activity Logs so users can see which task records still have screenshot backlog and use the existing task resync action to push those screenshots.

Keep the first implementation simple:

- local-first capture
- one orchestration path for upload readiness
- reuse the existing sync queue instead of inventing a second background subsystem
- no risky file-moving logic unless we truly need it

**Current Situation**

- `backend/service_classes/screenshot.service.go` captures screenshots locally, writes a `screenshots` row, then immediately tries Backblaze upload inside `trySync()`.
- Local files are currently stored under `storageRoot/{taskRecordID}/{taskID}/...`.
- `backend/services/backblaze.go` builds the remote object path from the immediate parent folder of the local file path. Because of the current local layout, the remote Backblaze folder is effectively the task ID, not `Productivity`.
- `TaskRecord.Productivity` is only assigned later in `backend/service_classes/sync.service.go` during `syncTaskIn()` after the `wf/task_in` response returns.
- `SyncTaskRecord()` is already the shared backend sync path used by active task sync and Activity Logs resync, which makes it the right place to centralize screenshot upload readiness checks.
- `SyncUnsyncedRecords()` only loads task records with `pending`, `failed`, or `blocked` sync status. That means a task record that is already FES-synced but still has pending screenshots would currently be skipped by the global "Sync now" flow.
- `backend/services/activity_log.go` currently returns attendance plus raw task records only. Activity Logs has no per-record screenshot backlog metadata today, so the frontend cannot tell the user that a task is synced to FES but still has pending screenshot uploads.
- `SyncService` already has a queue and dedupe mechanism. `BackblazeSync` also has an old `queue []func()` plus `attemptUploadQueued()` helpers, but that path is not the real orchestration layer today.
- `ScreenshotService.FolderName` and the `localStorageValue` passed from the frontend are no longer the real source of truth for upload folder naming.

**Requested Behavior, Made Concrete**

1. Screenshot capture should always succeed locally first.
2. If `TaskRecord.Productivity` is missing, do not upload yet.
3. The screenshot DB row must still be saved and stay linked to `task_record_id`.
4. On every later sync attempt for that task record, if `Productivity` is still missing, leave screenshots pending.
5. On every later sync attempt for that task record, if `Productivity` now exists, upload every screenshot row for that task record whose `cloud_url` is still empty.
6. This must work for active task sync and Activity Logs resync.
7. Recommended: cover the batch "Sync now" flow as well, so backlog does not get stranded.
8. Activity Logs should show the pending screenshot count for each task record when that count is greater than zero.
9. The Activity Logs task resync action should remain the way users retry screenshot backlog for that task record; no separate screenshot-only button is needed in v1.
10. Upload work should go through the existing sync queue so the logic is serialized and easier to reason about.

**Recommended Decisions Before Coding**

1. Remote folder semantics
- Recommended: make `Productivity` the Backblaze folder name only.
- Keep local files staged under a stable task-record-based path in v1.
- Reason: this satisfies the upload requirement without introducing local file renames, move failures, or path-rewrite cleanup logic.
- Alternate: rename local folders after `Productivity` arrives. This is riskier and not needed for the workflow to work.

2. Failure behavior
- Recommended: do not fail or downgrade the main FES task sync if only Backblaze upload fails.
- Leave screenshot rows pending and retry them on later sync attempts.
- Reason: otherwise a screenshot upload failure can force repeated `task_in` or `task_out` behavior and make sync state misleading.
- Alternate: fail the whole task-record sync when screenshot upload fails. This is simpler on paper, but much riskier in behavior.

3. Queue placement
- Recommended: run screenshot draining inside the existing queued `SyncTaskRecord()` job, not as a nested queued job.
- Reason: this avoids queue re-entry, deadlock risk, and dropped work when a screenshot arrives while the same task record is already syncing.
- Alternate: add a separate queued screenshot-upload job. This is workable, but only if it is fire-and-forget and cannot wait on itself.

4. Schema scope
- Recommended: keep the first pass schema-light and use empty `cloud_url` as the pending marker.
- Reason: the user asked not to overengineer, and we can ship the core behavior without a migration.
- Alternate: add `upload_status`, `last_upload_error`, and `uploaded_at` columns now. Useful later, but not required for the first pass.

5. Activity Logs visibility
- Recommended: add a lightweight derived field such as `PendingScreenshotCount` to the Activity Logs task-record payload and reuse the existing task resync action.
- Reason: this gives users a clear "why is this still not fully done?" signal without adding a new endpoint, a new button, or a new persisted screenshot status model.
- Alternate: build a separate screenshot-sync UI or endpoint. That adds complexity without much v1 value.

**Files Likely To Change**

- `backend/service_classes/screenshot.service.go`
  Purpose: remove direct upload orchestration from capture, keep the service local-first, and always hand off readiness decisions to `SyncService`.
- `backend/service_classes/sync.service.go`
  Purpose: make task-record sync responsible for checking and draining pending screenshots when `Productivity` exists.
- `backend/services/backblaze.go`
  Purpose: stop deriving the remote folder from the local path and accept an explicit folder name based on `Productivity`.
- `backend/services/activity_log.go`
  Purpose: enrich Activity Logs task-record payloads with pending screenshot counts.
- `backend/db/models/TaskRecord.go`
  Purpose: carry lightweight derived screenshot-backlog metadata to the frontend without adding a new persistence model.
- `backend/services/backblaze_test.go`
  Purpose: update path-builder coverage for the new folder naming rule.
- `backend/service_classes/screenshot.service_test.go`
  Purpose: cover pending-local behavior and backlog upload behavior.
- `backend/service_classes/sync.service_test.go`
  Purpose: cover the "has productivity -> drain screenshots" rules and already-synced record retry path.
- `frontend/src/pages/Main/ActivityLogs.tsx`
  Purpose: show pending screenshot counts and make the existing resync affordance clearly cover screenshot backlog too.

Optional only if the helpers start crowding existing files:

- `backend/service_classes/screenshot_upload.go`
  Purpose: small helper file for backlog loading and upload processing. Not a new subsystem.

**Implementation Plan**

1. Make `Productivity` the authoritative remote folder key.
- Add a helper that resolves and sanitizes the folder name from `taskRecord.Productivity`.
- Change the Backblaze upload API so the caller passes the remote folder explicitly.
- Use a deterministic collision-safe object format such as `screenshots/{userID}/{productivity}/{screenshotID}-{baseFilename}`.

2. Keep screenshot capture local-first.
- `Screenshot()` should continue writing files locally and inserting the `screenshots` row immediately.
- `ScreenshotService` should not decide whether `Productivity` is ready for upload.
- If the app is online, capture should always request `SyncTaskRecord(taskRecordID)`.
- Capture should never call Backblaze directly anymore.

3. Remove upload orchestration from `ScreenshotService.trySync()`.
- Delete the immediate Backblaze retry loop from the screenshot service.
- Keep only the "ask SyncService to sync this task record" behavior.
- This makes task-record sync the single source of truth for whether upload is allowed yet.

4. Add one pending-screenshot drain helper in `SyncService`.
- Add a helper such as `drainPendingScreenshotsNow(taskRecord *models.TaskRecord)`.
- It should run inline inside the already-queued `SyncTaskRecord()` job, not enqueue another job and wait for it.
- It should confirm the task record belongs to the authenticated user, confirm the app is online, confirm `Productivity` exists, query screenshot rows where `cloud_url` is null or empty, decode `file_path`, skip missing local files, upload each screenshot independently to the explicit `Productivity` folder, and update `cloud_url` only after that screenshot succeeds.
- If one screenshot upload fails, log it and continue processing the others.

5. Hook the drain into every relevant task-record sync path.
- If `SyncTaskRecord()` loads a record that already has `Productivity`, try draining pending screenshots before any early return that would otherwise stop the flow.
- If `syncTaskIn()` obtains `Productivity` during the current sync attempt, persist it first, reload the task record, then run the drain step.
- For ended records, preserve the current FES task sync ordering and run screenshot upload as best-effort after the FES sync work so a screenshot failure does not force another `task_out`.

6. Extend the batch sync path so screenshot backlog is not stranded.
- Update `SyncUnsyncedRecords()` so it also processes task records that are already `synced` but still have screenshot rows with empty `cloud_url`.
- Keep this simple: first process the normal unsynced task records, then load distinct task record IDs that still have pending screenshots for the current user, combine and dedupe the IDs, and route them back through the same `SyncTaskRecord(id)` entry point.
- This makes Profile Bar "Sync now" useful for Backblaze backlog too.

7. Extend the Activity Logs payload with screenshot backlog metadata.
- Add a lightweight derived field such as `PendingScreenshotCount` on task records returned to Activity Logs.
- Populate it from the `screenshots` table by counting rows for that `task_record_id` where `cloud_url` is null or blank.
- Keep this derived-only in v1; do not add new persisted screenshot status columns just for the UI.

8. Reflect screenshot backlog clearly in Activity Logs.
- For task records with `PendingScreenshotCount > 0`, show a note such as "`N` screenshots pending upload".
- Reuse the existing record resync action so users can retry the task record and its pending screenshots from the same row.
- When the row is in local frontend syncing state, show that the retry is syncing the task record plus pending screenshots.
- Avoid showing a misleading fully-clean "Synced" state when the task record is FES-synced but still has screenshot backlog.

9. Neutralize stale folder inputs.
- Stop treating `ScreenshotService.FolderName` or the frontend `localStorageValue` as Backblaze naming inputs.
- If they become dead after the refactor, remove or reduce them in the same pass only if the cleanup stays low-risk.
- Do not redesign the frontend contract beyond what the backend actually needs.

10. Add focused tests instead of broad rewrites.
- Backblaze path builder uses the explicit `Productivity` folder.
- Screenshot rows remain local with empty `cloud_url` when `Productivity` is missing.
- Screenshot capture still requests `SyncTaskRecord()` when online, but never calls Backblaze directly.
- A later task-record sync uploads all pending screenshots once `Productivity` exists.
- `syncTaskIn()` persists `Productivity` before the backlog drain runs.
- A task record that is already `synced` still retries pending screenshot uploads.
- Batch "Sync now" also drains screenshot backlog for already-synced records.
- Activity Logs payload includes `PendingScreenshotCount`.
- Activity Logs UI shows pending screenshot counts and keeps the existing resync path for retry.
- Missing local files are skipped without crashing the whole drain.

**Acceptance Checklist**

- Backblaze folder naming is driven by `TaskRecord.Productivity`, not task ID and not the local directory layout.
- Screenshot capture still works when `Productivity` is missing and saves the DB row locally with empty `cloud_url`.
- Screenshot capture no longer calls Backblaze directly and always routes retry readiness through `SyncTaskRecord()`.
- A later sync attempt for that task record uploads every pending screenshot row once `Productivity` exists.
- Active task sync and Activity Logs resync both reach the same upload behavior through backend `SyncTaskRecord()`.
- Batch "Sync now" can also recover screenshot backlog for records that are already FES-synced.
- Activity Logs shows pending screenshot counts for task records with backlog and lets the user retry them from the existing resync action.
- The refactor reuses the existing `SyncService` queue and does not add a second always-on queue system.

**Review / Testing Checklist**

- `go test ./backend/service_classes ./backend/services -count=1`
- If queue behavior needs end-to-end proof, also run `go test ./backend/app -count=1`
- Manual verification in dev: start a task before `Productivity` exists, capture screenshots, confirm `screenshots.cloud_url` stays empty, trigger sync so `Productivity` is populated, confirm pending screenshots upload and land under the `Productivity` folder in Backblaze, and confirm `cloud_url` is written afterward.
- Manual verification in Activity Logs: confirm a task record with screenshot backlog shows the pending count, confirm the existing resync action retries both the task record and its screenshots, and confirm the count clears after successful upload.
- Regression verification: active task with existing `Productivity` still captures and uploads, Activity Logs resync of a record with pending screenshots drains the backlog, and "Sync now" can recover pending screenshot uploads without needing manual Activity Logs resync.

**Success Criteria**

- The workflow has one clear readiness rule: no `Productivity`, no upload; `Productivity` exists, upload pending screenshots.
- Screenshot capture is local-first and no longer owns Backblaze retry logic.
- Activity Logs gives users a visible reason when a task record still needs screenshot upload work.
- Upload work is serialized through the existing sync queue, which keeps the flow cleaner and easier to debug.
- The implementation stays simple and avoids unnecessary schema changes or local file moves in the first pass.

**Risk Controls**

- Assume `TaskRecord.Productivity` is the correct remote folder identifier. If the backend/API actually expects a different ID, confirm that before coding.
- Keep local file staging unchanged in v1 to avoid risky folder rename logic.
- Do not let screenshot upload failures resubmit `wf/task_in` or `wf/task_out` unnecessarily.
- Do not make Activity Logs depend on a new persistent screenshot state machine for v1; derive pending counts from empty `cloud_url` instead.
- Treat empty `cloud_url` as the pending marker for phase 1; if support visibility later becomes important, add explicit upload-state columns in a follow-up migration.

**What Else I Suggest**

- Recommended follow-up, not phase 1: add a lightweight diagnostics view or export for "task records with pending screenshot uploads" if support needs visibility.
- Recommended follow-up, not phase 1: decide whether local files should be pruned after confirmed Backblaze upload. Right now the app still reads screenshots from local paths.
- Not recommended now: renaming local screenshot folders after `Productivity` arrives or building a second background worker just for Backblaze.
