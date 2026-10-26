# FES App Tracker

Desktop employee task and attendance tracker built with Wails, Go, React, and SQLite.

This README is the short entry point. The full maintainer and takeover documentation now lives in [`guide/`](guide/README.md).

## Start Here

If you are a new developer, read these first:

1. [`guide/README.md`](guide/README.md)
2. [`guide/takeover-playbook.md`](guide/takeover-playbook.md)
3. [`guide/architecture.md`](guide/architecture.md)
4. [`guide/developer-workflow.md`](guide/developer-workflow.md)

Area-specific guides:

- [`guide/backend.md`](guide/backend.md)
- [`guide/frontend.md`](guide/frontend.md)
- [`guide/data-and-sync.md`](guide/data-and-sync.md)
- [`guide/troubleshooting.md`](guide/troubleshooting.md)
- [`guide/glossary.md`](guide/glossary.md)
- [`guide/stable-release-workflow.md`](guide/stable-release-workflow.md) — developer branches, production tags, publishing, and user updates

## What The App Does

FES App Tracker is a desktop time-tracking application for First Encoding Services. It allows users to:

- log in online against the FES API or offline from previously stored local accounts
- start and stop task tracking sessions
- capture screenshots while work is running
- persist attendance and task records locally in SQLite
- retry synchronization when the app returns online
- upload screenshot artifacts through the screenshot sync pipeline when the required remote state is available
- review daily activity logs and manual resync options

The app is designed around local-first operation. The local database is not just a cache. It is the durable runtime record used for offline work and sync recovery.

## Tech Stack

### Backend

- Go
- Wails
- GORM
- SQLite

### Frontend

- React
- TypeScript
- Vite
- Zustand
- Tailwind CSS
- Radix UI

### Integrations

- FES API
- GitHub Releases for app updates
- OS keyring for secret storage
- Backblaze B2 for screenshot upload

## Documentation Map

The detailed handover docs live under [`guide/`](guide/README.md).

| File | Purpose |
| --- | --- |
| [`guide/README.md`](guide/README.md) | Main orientation, mental model, and reading order |
| [`guide/takeover-playbook.md`](guide/takeover-playbook.md) | First-day and first-week takeover plan |
| [`guide/architecture.md`](guide/architecture.md) | System boundaries, startup, shutdown, and runtime flow |
| [`guide/backend.md`](guide/backend.md) | Go backend structure, services, repositories, sync engine, and integrations |
| [`guide/frontend.md`](guide/frontend.md) | React app structure, state, pages, and Wails event usage |
| [`guide/data-and-sync.md`](guide/data-and-sync.md) | SQLite schema, sync states, record lifecycle, and screenshot pipeline |
| [`guide/developer-workflow.md`](guide/developer-workflow.md) | Setup, commands, testing, release workflow, and safe change patterns |
| [`guide/troubleshooting.md`](guide/troubleshooting.md) | Incident debugging checklists and common failure modes |
| [`guide/glossary.md`](guide/glossary.md) | Plain-language definitions of technical and project terms |

## Repository Layout

```text
wails-app/
|-- backend/      Go runtime, app facade, services, repositories, database
|-- frontend/     React application and generated Wails bindings
|-- guide/        detailed developer documentation
|-- build/        packaging assets and outputs
|-- plan/         planning artifacts
|-- main.go       Wails entry point
|-- wails.json    Wails app configuration
`-- README.md     this file
```

## Development

### Prerequisites

- Go toolchain matching `go.mod`
- Node.js
- npm
- Wails CLI

Install Wails if needed:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Run locally

```bash
wails dev
```

### Build

```bash
wails build
```

### Run backend tests

```bash
go test ./backend/...
```

## Architecture Snapshot

```text
React UI
  -> generated Wails bindings
    -> backend/app.App
      -> service classes
        -> repositories
          -> SQLite

backend/app.App
  -> FES API
  -> sync queue
  -> screenshot capture
  -> Backblaze upload
  -> desktop-specific runtime features
```

The most important backend boundary is `backend/app.App`. The most important operational subsystem is `backend/service_classes/sync.service.go`.

## Current Realities To Know Before Editing

- `backend/services` and `backend/service_classes` overlap in responsibility. Inspect call sites before refactoring either package.
- The app is local-first. A successful UI action does not always mean the remote FES system is already in sync.
- Attendance state is an upstream dependency for task sync. If attendance is not ready, task sync may block.
- Screenshot behavior spans multiple layers: task tracking, local file storage, SQLite metadata, sync reconciliation, and Backblaze upload.
- Version strings are not currently aligned across all files.
- Firebase exists in the frontend, but the main working login flow is driven by Wails calls into the Go backend and the FES API.

These details are documented more fully in [`guide/README.md`](guide/README.md).

## Notes For Maintainers

- Do not edit `frontend/wailsjs/` manually. Those files are generated.
- Treat the local SQLite database as important user data.
- Be careful with sync-state fields such as `pending`, `failed`, and `blocked`.
- Read the guide before changing auth, tracking, migrations, screenshot behavior, or sync logic.

## License

Internal project for First Encoding Services unless the repository owner specifies otherwise.
