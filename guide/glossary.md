# Glossary

This glossary explains technical and project-specific terms used across the guide.

It is intentionally beginner-friendly. If a word sounds obvious to experienced engineers but confusing to a new maintainer, it belongs here.

## A

### API

Application Programming Interface.

In this project, "the API" usually means the remote FES backend the desktop app talks to over HTTP.

### App facade

A simplified outer layer that exposes useful methods while hiding deeper internal complexity.

In this repo, `backend/app.App` is the facade exposed to the frontend by Wails.

### Attendance

A record representing one work session envelope, usually from login to logout.

Task records belong to an attendance.

## B

### Backblaze B2

A cloud object storage service used here for screenshot upload.

If screenshots sync remotely, their files may be uploaded to Backblaze while metadata stays in SQLite.

### Binding

A generated bridge function that lets the React frontend call Go methods directly through Wails.

## C

### Cache

Stored data kept so it can be reused later.

Important nuance in this project:

the local SQLite database is not merely a cache. It is durable operational state.

### Canonical schema

The approved, intended version of the database structure that migrations try to enforce.

### Client

In this project, a "client" usually means a company/customer entity from the remote FES system, not the frontend application.

### Concurrency

When different pieces of work can run during overlapping periods instead of one strict step after another.

The sync queue and screenshot worker are examples of concurrent behavior.

### Correlation ID

A generated ID added to related log lines so one operation can be followed more easily across multiple messages.

## D

### Diagnostic bundle

A support artifact the app can export that collects useful debugging information such as sync state and logs.

### DTO

Data Transfer Object.

A simplified shape used to move data between layers.

In this repo, exported structs like `ExpTask` behave like DTOs.

## E

### Event

A message emitted by the backend and listened to by the frontend.

Examples:

- `record-synced`
- `attendance-synced`
- `screenshot`

Events are important because not every backend action finishes before the original frontend call returns.

## F

### Facade

A simplified outer interface around more complex internals.

### Firebase auth

Authentication state provided by Firebase SDK in the frontend.

Important project-specific note:

Firebase auth appears in this repo, but the main operational login flow is primarily handled through Go and the FES API.

### Foreign key

A database relationship that connects one table's row to another.

Example:

`task_records.attendance_id` points to an attendance row.

## G

### GORM

The Go ORM library used in this project.

ORM means "Object Relational Mapper", which is a tool that maps structs in code to database tables.

## H

### Hot query

A query that runs frequently or is performance-sensitive.

The codebase creates indexes for hot queries to keep them efficient.

## I

### Idempotent

Safe to run multiple times without causing different results after the first successful run.

Migration and sync helper code often aims for idempotent behavior.

### Index

A database structure that helps certain lookups run faster.

Good indexes are especially important for task history, attendance history, and sync-status queries.

## J

### Job

A unit of background work.

In the sync service, a job might represent "sync this attendance" or "sync this task record".

## K

### Keyring

The operating system's secure credential storage.

This app uses it for secrets such as:

- FES API token
- Backblaze credentials

## L

### Local-first

A design style where the app writes and trusts local state first, then synchronizes to remote systems later when possible.

This is one of the most important concepts in the whole project.

### `login_logout_id`

A remote identifier returned by the FES system for an attendance session.

Task sync often depends on this existing first.

## M

### Migration

A controlled database change or repair step.

In this project, migrations are more careful than simple auto-generated schema updates.

### Model

A Go struct representing stored data or an exported data shape.

Persistence models live under `backend/db/models/`.

## O

### Offline mode

A state where the app continues working locally even though remote sync cannot proceed right now.

### Operational source of truth

The layer you should trust most for real current state.

In many important flows here, that is the local SQLite database until remote sync completes.

### ORM

Object Relational Mapper.

A library that lets code work with structs and queries rather than raw SQL only.

## P

### Padagan

A project-specific work classification stored on task records as a boolean.

If you are not already familiar with the business meaning, treat it as a special mode that affects tracking and summaries.

### Pending

A sync state meaning the record still needs remote synchronization.

### Pointer

A Go value that references another value's memory location.

This project uses shared pointers between services for active session state, so pointer behavior matters during refactors.

### Productivity

The remote identifier associated with a synced task record in the FES system.

Task-out sync and screenshot upload correlation may depend on it.

## Q

### Queue

A structure where work items wait to be processed one at a time or in controlled order.

The sync service uses a queue to prevent chaotic overlapping sync attempts.

## R

### Record

A stored unit of data, usually one row in a database table.

### Reconciliation

The act of checking and aligning local state with the remote state needed to consider something truly synced.

### Repository

A layer that encapsulates database access.

Repositories are meant to keep raw persistence logic separate from higher-level business orchestration.

### Retry backoff

A retry strategy where repeated failures wait longer between attempts.

The sync queue uses this idea for retryable failures.

## S

### Screenshot backlog

Screenshot rows whose local files exist but whose remote upload has not completed yet.

### Service

Code that performs application logic.

Important project-specific warning:

this repo has both `backend/service_classes` and `backend/services`, and both matter.

### Single-instance lock

A desktop safeguard that prevents multiple copies of the app from running at the same time.

### Source of truth

The layer that should be trusted most for a specific piece of information.

### Stable error

An error intentionally shaped to be safe for returning to the frontend.

### Sync

Short for synchronization.

It means sending or reconciling local data with remote systems.

### `sync_status`

A database field describing the current synchronization state of a record.

### Systray

The small app icon/menu that lives in the operating system tray area.

## T

### Task

A project/work item the user can track time against.

### Task record

A local record representing one tracked segment of time for one task.

### Transitional code

Code that still works and may still be used, even though the architecture is moving toward a cleaner shape.

Much of `backend/services` should be treated this way.

## U

### Upwork helper flow

A desktop-specific behavior in the app that fetches credentials and triggers browser/app automation for certain tasks.

## V

### WAL mode

SQLite "write-ahead logging" mode.

You do not need to be a database expert to remember the important part:

it is a more robust SQLite operating mode than the simplest default.

### WebView

The embedded browser runtime inside the desktop window.

The React frontend renders inside this WebView.

## W

### Wails

The framework used to build this app with a Go backend and web frontend inside a desktop shell.

### Recovery

The act of safely handling a panic or failed sync so the app can continue or leave enough state behind for later retry.

## Read next

If a glossary term now makes sense but the full system still feels fuzzy, return to [`guide/architecture.md`](./architecture.md).
