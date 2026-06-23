# Production Readiness Bug Fix Plan

Generated on 2026-06-24 after reviewing the current implementation, the existing `plan/REFACTOR.md`, backend test status, frontend wiring, sync flows, lifecycle handling, and release configuration.

## Purpose

This document is narrower than `plan/REFACTOR.md`.

It focuses on the bug fixes and hardening work required to ship a production-ready app without requiring a broad architectural rewrite first.

Non-goals for this plan:

- Full package reorganization.
- Clean architecture migration.
- Renaming all events and bindings before release.
- Frontend redesign.
- Replacing Wails, GORM, or SQLite.

## Clarified Goal

Ship the current app safely enough for production by eliminating release-blocking failures in:

- lifecycle and shutdown behavior
- sync correctness and duplicate remote writes
- nil/out-of-sequence binding calls
- screenshot behavior if screenshots are part of the release
- updater and release metadata consistency

## Current Situation

- `go test ./...` passes as of 2026-06-24.
- There is no frontend automated test setup in `frontend/package.json`.
- `plan/REFACTOR.md` is broader than what is required to release safely.
- The app still mixes active and legacy sync paths:
  - `backend/service_classes/sync.service.go`
  - `backend/services/sync.go`
- Lifecycle handling is still incomplete:
  - `backend/app/app.go`
  - `frontend/src/App.tsx`
- Screenshot capture is currently disabled in code:
  - `backend/service_classes/screenshot.service.go`
- Version metadata is inconsistent:
  - `backend/app/app.go`
  - `wails.json`
  - `README.md`

## Release-Blocking Findings

### 1. Lifecycle and shutdown behavior are still unsafe

Files:

- `backend/app/app.go`
- `frontend/src/App.tsx`
- `backend/service_classes/sync.service.go`

Current risks:

- `NewApp()` still calls service `Connect()` methods before Wails `Startup()` assigns the runtime context.
- The sync queue can fall back to `context.Background()`, which weakens shutdown ownership.
- The frontend still listens for `app_shutdown` and also calls `StopTracking()`, while backend shutdown already stops tracking.
- Systray startup exists, but shutdown ownership is still incomplete.

Production impact:

- duplicate stop behavior
- incomplete cleanup
- background workers outliving normal app shutdown
- harder-to-debug task/session corruption during close

### 2. Sync behavior is richer than before but not yet release-safe

Files:

- `backend/service_classes/sync.service.go`
- `backend/services/sync.go`
- `backend/app/app.go`

Current risks:

- The newer sync service tracks attempts and errors, but the legacy Firebase sync path still exists and is still reachable.
- `SyncTaskRecord()` does not yet enforce a complete idempotency model for task-in/task-out.
- `SyncAllAttendance()`, `SyncUser()`, and `RefreshAccessToken()` are still stubs in the newer sync service.
- `SyncActiveTaskRecord()` is incomplete and not clearly production-safe.

Production impact:

- duplicate or conflicting remote records
- ambiguous source of truth
- maintenance risk during incident response

### 3. Wails bindings still allow nil or out-of-sequence state access

Files:

- `backend/app/app.go`

Current risks:

- Some binding methods still assume `AuthUser`, `Attendance`, or active session state already exists.
- Generated frontend bindings still expose `BeforeClose(context.Context)`, which should not be part of a frontend-invoked contract.

Production impact:

- crash-class nil dereferences
- unstable behavior after logout, app restore, or invalid navigation flow

### 4. Screenshot behavior needs an explicit release decision

Files:

- `backend/service_classes/screenshot.service.go`
- `frontend/src/pages/Main/Task.tsx`
- `README.md`

Current risks:

- Screenshot capture is disabled in code.
- The UI still assumes screenshot-related behavior exists.
- Storage still uses a relative app path rather than a user data directory.

Production impact:

- required feature silently missing
- mismatch between documented behavior and shipped behavior
- incomplete support path when users report screenshot issues

### 5. Updater and version metadata are not ready for release discipline

Files:

- `backend/app/app.go`
- `wails.json`
- `README.md`
- `frontend/src/App.tsx`

Current risks:

- `DownloadAndInstall()` still uses direct `os.Exit(0)` after launching the installer.
- `CurrentVersion`, `wails.json`, and `README.md` disagree.
- Update checks are still triggered from immediate frontend mount.

Production impact:

- skipped cleanup during update
- unclear release identity
- harder support and rollback workflows

## Recommended Scope For Production

Treat the following as mandatory before release:

1. Shutdown and lifecycle correctness.
2. Single active sync path for production flows.
3. Binding/session nil guards and stable client errors.
4. Screenshot release decision and alignment.
5. Updater shutdown safety and version consistency.

Treat the following as deferrable until after release:

- package restructuring
- `frontend/src/api/` abstraction layer
- event renaming to a new contract
- broader repository constructor cleanup
- frontend test architecture beyond minimal release checks

## Files Likely To Change

- `backend/app/app.go`
  - move runtime-dependent wiring to startup-safe paths, tighten shutdown, remove unsafe binding surface where possible
- `backend/service_classes/sync.service.go`
  - finish production-safe sync behavior and quarantine incomplete paths
- `backend/service_classes/auth.service.go`
  - verify logout/session flow against the shutdown and sync rules
- `backend/service_classes/screenshot.service.go`
  - align screenshot behavior with the release decision
- `frontend/src/App.tsx`
  - remove duplicate shutdown stop logic and reduce updater coupling at mount
- `frontend/src/pages/Main/Home.tsx`
  - ensure UI sync state reflects backend truth for release-critical flows
- `frontend/src/pages/Main/Task.tsx`
  - align tracking and screenshot UI with actual backend state
- `frontend/wailsjs/go/app/App.d.ts`
  - regenerate after binding changes if backend exported methods change
- `frontend/wailsjs/go/app/App.js`
  - regenerate after binding changes if backend exported methods change
- `frontend/wailsjs/go/models.ts`
  - regenerate if exported Go models change
- `wails.json`
  - align release metadata
- `README.md`
  - align documented shipped behavior with actual release behavior

## Production Fix Phases

### Phase 0: Release gate and feature decisions

Goal: lock the actual production feature set before code changes continue.

Tasks:

- Decide whether screenshots are required in the upcoming release.
- Decide whether the legacy Firebase sync path is still supported in production or should be disabled from active flows.
- Confirm one production source of truth for versioning.

Acceptance:

- There is a written yes/no decision for screenshots in the release.
- There is a written yes/no decision for legacy Firebase sync availability.
- Release version ownership is defined in one place.

### Phase 1: Lifecycle and shutdown correctness

Goal: close the app without duplicate task stop behavior, leaked workers, or inconsistent runtime state.

Tasks:

- Stop calling runtime-dependent `Connect()` methods with nil context from `NewApp()`.
- Ensure the sync worker starts from a Wails-owned context path.
- Remove frontend shutdown `StopTracking()` once backend shutdown is authoritative.
- Define shutdown ordering:
  - stop screenshot worker
  - stop active task once
  - stop sync queue with timeout/cancel
  - stop systray
- Review `BeforeClose()` exposure and keep it out of frontend-driven flows.

Acceptance:

- Closing the app stops the active task once.
- No frontend shutdown callback triggers duplicate backend stop behavior.
- Runtime workers do not require `context.Background()` to survive.

### Phase 2: Production-safe sync path

Goal: keep local records intact and prevent duplicate or ambiguous remote sync results.

Tasks:

- Make the newer `SyncService` the only active production sync path for current flows, or explicitly disable legacy Firebase paths in `App`.
- Define idempotency rules for:
  - attendance login sync
  - attendance logout sync
  - task-in sync
  - task-out sync
- Ensure retrying sync does not create duplicate remote entries when a remote identity already exists.
- Either complete `SyncActiveTaskRecord()` for production usage or remove it from the release path.
- Keep stable local state when remote sync fails.

Acceptance:

- Re-running sync on the same attendance/task record does not duplicate remote writes.
- Offline records remain local and retryable.
- Legacy sync code is not part of the active release path unless explicitly kept and tested.

### Phase 3: Binding and session guards

Goal: prevent crash-class failures from invalid frontend call order.

Tasks:

- Add guardrails around bindings that depend on `AuthUser`, `Attendance`, or active session state.
- Return stable UI-safe errors instead of relying on implicit nil assumptions.
- Remove or quarantine frontend-visible methods that should not be user-invoked, especially `BeforeClose()`.

Acceptance:

- Calling session-dependent bindings while logged out returns stable errors instead of crashing.
- Frontend-generated bindings no longer imply unsupported frontend lifecycle calls.

### Phase 4: Screenshot release alignment

Goal: either ship screenshots safely or stop presenting them as an active production feature.

Tasks if screenshots are required:

- Re-enable screenshot capture intentionally, not accidentally.
- Store screenshots in a user data directory.
- Verify start/stop/start behavior for the worker.
- Ensure screenshot failures do not block task stop or logout.
- Surface screenshot status clearly enough for release support.

Tasks if screenshots are not required:

- Keep screenshots disabled intentionally.
- Remove or hide release-visible screenshot expectations in UI and docs.

Acceptance:

- The shipped app behavior matches the release decision.
- There is no mismatch between code, UI, and README about screenshot support.

### Phase 5: Updater and release metadata hardening

Goal: make update/install behavior safe and make release identity consistent.

Tasks:

- Replace direct `os.Exit(0)` with a shutdown-aware flow after installer launch.
- Align `CurrentVersion`, `wails.json`, and README versioning.
- Delay or guard update checks so startup/login/tracking do not depend on updater timing.
- Verify asset selection and installer path behavior for the actual Windows release artifact.

Acceptance:

- Update flow does not bypass cleanup.
- Release version is consistent across runtime, config, and docs.
- Updater failure does not block normal app usage.

## Implementation Rule

Keep the production pass minimal and behavior-focused:

- do not reorganize packages unless the bug fix requires it
- do not introduce new architecture layers just to satisfy the older refactor plan
- fix the active release path first
- quarantine incomplete code paths rather than partially supporting them

## Testing Surfaces

### Automated

Run:

- `go test ./...`
- `npm run build` from `frontend`

Add or update backend tests for:

- app shutdown stopping active task once
- sync retry without duplicate remote records
- logged-out binding guards
- screenshot-required or screenshot-disabled release path
- updater/version guard behavior where practical

### Manual smoke checks

- Online login succeeds.
- Offline login still works when expected.
- Start task, stop task, logout, and close app behave consistently.
- Closing the app with an active task does not double-stop it.
- Offline tracking creates retryable local records.
- Reconnect sync does not duplicate remote attendance or task entries.
- Activity log resync follows ownership and time window rules.
- Update check failure does not block the app.
- Version shown by the app matches the packaged release metadata.

## Acceptance Checklist

- No known crash path remains from nil session/binding state in normal UI flows.
- App shutdown is single-owner and deterministic.
- Only one production sync path is active for current release flows.
- Sync retries are idempotent enough to avoid duplicate remote records.
- Screenshot behavior is intentionally shipped or intentionally absent.
- Update flow no longer bypasses cleanup.
- Release version metadata is consistent.
- Backend tests pass and frontend builds successfully.

## Success Criteria

This plan is complete when a release candidate can be built and the current app can:

- log in
- track work
- stop work
- sync or retry safely
- close safely
- update safely

without relying on unresolved refactor work or ambiguous legacy paths.

## Risk Controls

- Preserve the existing user-facing flow unless a fix removes a proven defect.
- Prefer feature gating or quarantining incomplete paths over partial rewrites.
- Regenerate Wails bindings only after backend export changes are finalized.
- Keep the broad refactor work in `plan/REFACTOR.md` as a follow-up track, not a release blocker by default.

## Deferred Work After Production

- package structure cleanup
- frontend API wrapper layer
- event contract redesign
- broader repository constructor injection
- additional frontend tests and hooks consolidation
- long-term architecture cleanup from `plan/REFACTOR.md`
