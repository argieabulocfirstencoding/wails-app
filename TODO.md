# FESApp Enterprise Reliability & Security TODO

Last updated: 2026-05-29

## How to use this plan
- Work top-to-bottom by priority (`P0` first).
- Treat each checkbox as a deliverable with code + tests + verification notes.
- Do not ship a new production build until all `P0` items are done.

## P0 Release Blockers (Stability + Security)

### 1) Fix runtime-critical crashes
- [x] Remove panic-prone type assertions in auth/login and replace with validated parsing.
- [x] Guard all nil-sensitive task/record flows (especially start/stop tracking paths).
- [x] Make update-check parsing safe for empty/invalid release payloads.
- [x] Replace all `log.Fatalf`/`panic` in runtime paths with recoverable errors and user-safe behavior.
- [x] Add a global panic recovery wrapper for background goroutines (sync, tray, screenshot).

### 2) Restore screenshot capture reliability
- [ ] Decide single source of truth: keep only one screenshot service implementation.
- [x] Ensure `Start/Stop/Screenshot/AddMemo` are fully functional in the active service class.
- [x] Add cancellation-safe screenshot loop tied to app lifecycle/context.
- [x] Add failure backoff + retry for capture and upload path.
- [ ] Add end-to-end manual test script for multi-monitor + offline + online.

### 3) Secure updater pipeline
- [x] Restrict updates to trusted release source only (allowlist host/path).
- [x] Verify installer integrity before execute (checksum and/or signature verification).
- [x] Require HTTPS + strict status/content checks for download.
- [x] Prevent arbitrary URL install from UI-triggered calls.
- [x] Log updater decisions (version, URL, checksum result, execute result).

### 4) Hardening sync correctness
- [x] Keep task sync blocked until attendance dependency is satisfied (`login_logout_id` or synced attendance).
- [x] Add explicit sync state machine (`pending`, `in_progress`, `synced`, `failed`, `blocked`).
- [x] Persist retry metadata (`attempt_count`, `last_error`, `last_attempt_at`) for audits.
- [x] Ensure queue jobs are idempotent and deduplicated per logical record.
- [x] Add reconciliation check: verify external backend reflects sync before final “synced” mark.

### 5) Production-safe error handling + DB writes
- [x] Remove ignored DB write errors in repositories/services.
- [x] Add transaction boundaries where multiple updates must commit atomically.
- [x] Standardize error wrapping with operation context (`op`, `record_id`, `user_id`).
- [x] Return stable error shapes to frontend (no raw internal stack text).

## P1 Reliability Foundations

### 6) Remove global mutable runtime state risk
- [ ] Replace package-level `OnlineMode` global with scoped connectivity/session state service.
- [ ] Pass connectivity via context/service dependencies instead of global variable access.
- [ ] Add tests for online/offline transitions during active task + sync queue processing.

### 7) Observability and diagnostics
- [ ] Implement logger levels (`Info/Warn/Error/Debug`) with consistent formatting.
- [ ] Add correlation IDs for login, attendance sync, task sync, screenshot batch.
- [ ] Add log rotation/retention policy for production installs.
- [ ] Add “Export Diagnostic Bundle” action (logs + app version + OS + sanitized sync states).

### 8) Data integrity and migration safety
- [ ] Add schema migration versioning and rollback strategy.
- [ ] Add migration tests for upgrade paths from older released DB versions.
- [ ] Add foreign key and index review for hot queries (`attendance`, `task_records`, `screenshots`).
- [ ] Validate timezone handling consistency for activity logs and summaries.

## P2 Security Hardening (Enterprise Controls)

### 9) Secrets and credential handling
- [ ] Move all sensitive keys/tokens to secure storage + environment-injected config.
- [ ] Remove hardcoded test credentials from repository and history.
- [ ] Rotate exposed credentials and document rotation cadence.
- [ ] Enforce token lifetime checks and secure refresh flows.

### 10) API and IPC boundary hardening
- [ ] Validate/sanitize all bound method inputs (length, format, required fields).
- [ ] Reduce exposed Wails bindings to only supported production endpoints.
- [ ] Add permission checks before sensitive operations (open web app, updater, sync actions).
- [ ] Add rate limiting/debouncing for high-frequency frontend->backend operations.

### 11) Secure release and supply chain
- [ ] Use signed releases and verify build artifacts in CI.
- [ ] Add dependency auditing (Go + npm) with blocking policy for critical CVEs.
- [ ] Pin dependency versions and track upgrade policy.
- [ ] Add SBOM generation for each production release.

## P3 Test, QA, and Operational Readiness

### 12) Testing strategy
- [ ] Rename/fix non-running test files so CI executes them (`*_test.go`).
- [ ] Add unit tests for sync queue, retry logic, attendance/task dependency gating.
- [ ] Add repository tests with temp DB for critical query/update behavior.
- [ ] Add frontend tests for activity logs error states and partial-load behavior.
- [ ] Add smoke tests for login -> start task -> screenshot -> stop -> sync -> logout.

### 13) CI/CD and quality gates
- [ ] Add CI pipeline with gates: format, lint, tests, build, security scan.
- [ ] Fail builds on race conditions (`go test -race`) for backend concurrency hotspots.
- [ ] Add artifact packaging tests for installer and portable binary.
- [ ] Require changelog + migration note + rollback note per release.

### 14) Runbook and support readiness
- [ ] Create incident runbook (sync stuck, login failure, screenshot failure, update failure).
- [ ] Create rollback playbook for bad releases.
- [ ] Define SLOs: login success, sync success latency, crash-free sessions.
- [ ] Define on-call checklist and escalation policy.

## Milestones

### M1: “Safe to pilot internally”
- [ ] All `P0` tasks complete.
- [ ] Critical sync and screenshot flows tested on at least 3 Windows machine profiles.
- [ ] Updater integrity verification enabled.

### M2: “Enterprise-ready beta”
- [ ] All `P1` tasks complete.
- [ ] CI quality gates enforced on main branch.
- [ ] Diagnostic bundle and runbooks available for support.

### M3: “Production enterprise release”
- [ ] All `P2` and `P3` high-priority items complete.
- [ ] Security review sign-off.
- [ ] Release readiness checklist signed by engineering + operations.

## Tracking Template (copy per sprint)
- Sprint:
- Goal:
- Scope (task IDs):
- Risks:
- Done:
- Validation evidence:
- Follow-ups:
