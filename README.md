# FES App Tracker

Desktop employee task and attendance tracker built with Wails, Go, React, and SQLite.

This README is the short entry point. The detailed maintainer documentation now lives in [`guide/`](guide/README.md).

## Start Here

If you are a new developer, read these first:

1. [`guide/README.md`](guide/README.md)
2. [`guide/architecture.md`](guide/architecture.md)
3. [`guide/developer-workflow.md`](guide/developer-workflow.md)

Area-specific guides:

- [`guide/backend.md`](guide/backend.md)
- [`guide/frontend.md`](guide/frontend.md)
- [`guide/data-and-sync.md`](guide/data-and-sync.md)
- [`guide/glossary.md`](guide/glossary.md)

## What The App Does

FES App Tracker is a desktop time-tracking application for First Encoding Services. It allows users to:

- log in online against the FES API or offline from previously stored local accounts
- start and stop task tracking sessions
- persist attendance and task records locally in SQLite
- retry synchronization when the app returns online
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
- Backblaze B2 plumbing exists in code, but screenshot upload is currently disabled at the backend constant level

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
  -> frontend/wailsjs generated bindings
    -> backend/app.App
      -> service_classes
        -> repository
          -> SQLite

backend/app.App
  -> FES API integration
  -> sync queue
  -> local storage helpers
  -> desktop-specific runtime features
```

The most important backend boundary is `backend/app.App`. The most important operational subsystem is `backend/service_classes/sync.service.go`.

## Current Realities To Know Before Editing

- `backend/services` and `backend/service_classes` overlap in responsibility. Inspect call sites before refactoring either package.
- Screenshot behavior is inconsistent across layers. Frontend release flags say enabled, but backend capture and Backblaze upload constants are disabled.
- Version strings are not currently aligned across all files.
- Firebase exists in the frontend, but the main working login flow is driven by Wails calls into the Go backend and the FES API.

These details are documented more fully in [`guide/README.md`](guide/README.md).

## Notes For Maintainers

- Do not edit `frontend/wailsjs/` manually. Those files are generated.
- Treat the local SQLite database as important user data.
- Be careful with sync-state fields such as `pending`, `failed`, and `blocked`.
- Read the guide before changing auth, tracking, migrations, or sync logic.

## License

Internal project for First Encoding Services unless the repository owner specifies otherwise.
