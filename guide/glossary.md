# Glossary

This glossary explains the technical terms and project-specific words used in the rest of the guide.

## General software terms

### API

Application Programming Interface. In this project, it usually refers to the remote FES backend that the desktop app syncs with.

### Binding

A generated function bridge between the React frontend and the Go backend. Wails generates these under `frontend/wailsjs/`.

### Canonical schema

The single approved database table structure the migrations are trying to enforce, even when older local databases have drifted.

### DTO

Data Transfer Object. A simplified data shape used for passing data between layers or across process boundaries. In this project, `ExpTask`, `ExpTaskRecord`, and similar structs behave like DTOs.

### Facade

A simplified interface hiding more complex internals. `backend/app.App` is the Wails-facing facade.

### GORM

The Go ORM used to map structs to SQLite tables and run database operations.

### Idempotent

Safe to run multiple times without changing the result after the first successful run. The migration system tries to be idempotent.

### Migration

A controlled database schema change or repair step.

### ORM

Object Relational Mapper. A library that maps programming-language structs to database tables.

### Repository

A persistence-layer object that reads and writes data to the database.

### Retry backoff

A pattern where failed jobs are retried after increasing delays.

### Source of truth

The layer whose value should be trusted most for a given piece of information.

### Stable error

An error intentionally shaped for safe display to the frontend, without leaking too much internal detail.

## Wails and desktop terms

### Wails

The framework used to build this desktop app with a Go backend and web frontend.

### Systray

The system tray icon/menu integration used to keep the app available even when the window is hidden.

### Single-instance lock

A desktop safeguard that prevents multiple copies of the app from running at the same time.

### WebView

The embedded browser-like runtime that renders the React frontend inside the desktop app window.

## Project-specific terms

### Attendance

A local record representing one login/logout work session.

### LoginLogoutID

The remote identifier returned by the FES system for an attendance session. Task sync depends on this being available or the attendance already being recognized as synced.

### Productivity

The remote identifier for a synced task record on the FES side. Task-out sync and some screenshot correlation depend on it.

### Padagan

A project-specific tracking mode or work classification used in task records and summaries. It is stored as a boolean on `TaskRecord`.

### Break task

A task whose project name is `"Break"`. Break tasks are treated differently in some flows, including screenshot behavior and immediate sync decisions.

### Local mode

The UI concept that the user is operating from local/offline data and still needs to sync records upstream.

### Pending screenshot

A screenshot record whose local file metadata exists but whose `cloud_url` is still empty.

### Resync window

A guarded time period, currently 48 hours for attendance resync, during which the user is allowed to retry a remote sync operation.

## Sync-state terms

### Pending

The record exists locally and should be synced when possible.

### In progress

The sync worker is currently attempting to process the record.

### Synced

The remote operation succeeded and the local record has been marked complete.

### Failed

An actual sync attempt happened and returned an error.

### Blocked

The worker refused to continue because a required precondition was missing, such as offline state or unsatisfied attendance dependency.

## Storage terms

### WAL mode

SQLite write-ahead logging mode. It improves concurrency and durability behavior compared with simpler journal modes.

### Hot query index

An index created specifically for frequently used or performance-sensitive query patterns.

### Keyring

The OS credential storage used for secrets such as API tokens and Backblaze keys.

## Practical debugging vocabulary

### Correlation ID

A generated identifier attached to a chain of log messages so related operations can be grouped during debugging.

### Recovery

The act of safely handling a panic or failed sync so the app can continue or leave enough state behind for later retry.

### Reconciliation

The process of making sure local record state matches the minimum remote state required to treat an operation as successfully synced.
