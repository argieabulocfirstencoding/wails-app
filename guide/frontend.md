# Frontend Deep Dive

This document explains the React side of the Wails app.

## 1. Frontend stack

Main technologies:

- React 18
- TypeScript
- Vite
- React Router
- Zustand
- Tailwind CSS
- Radix UI primitives

The frontend lives under `frontend/src/`.

## 2. Entry points

### `frontend/src/main.tsx`

This file:

- creates the React root
- mounts `App`
- wraps the app in `ErrorBoundary`
- wraps the app in `ThemeProvider`

### `frontend/src/App.tsx`

This file:

- installs global hooks:
  - `useFirebaseAuth()`
  - `useOnlineStatus()`
- registers the router
- performs delayed release update checks when enabled

## 3. Routing structure

Routes currently defined:

| Route | Page | Purpose |
| --- | --- | --- |
| `/` | `Landing.tsx` | splash/loading redirect |
| `/login` | `Auth/Login.tsx` | online login |
| `/offline-login` | `Auth/OfflineLogin.tsx` | local-only login |
| `/home` | `Main/Home.tsx` | task list and current session status |
| `/task/:id` | `Main/Task.tsx` | per-task tracking screen |
| `/account` | `Account/AccountSettings.tsx` | account and local settings |
| `/notifications` | `Main/Notifications.tsx` | notification preferences UI |
| `/activity-logs` | `Main/ActivityLogs.tsx` | activity history and resync tools |
| `/logout` | `Auth/Logout.tsx` | logout confirmation flow |

Fallback route:

- `*` redirects to the login screen component

## 4. State management

Primary shared state lives in:

- `frontend/src/store/AuthContext.ts`

This is actually a Zustand store, not a React Context object, despite the filename.

### Important store fields

- `user`
- `attendance`
- `online`
- `taskID`
- `tasks`
- `summaries`
- `synced`
- `attendanceSynced`
- `fetchedActiveRecord`
- `initializedTask`

### Why this store matters

Most screens infer session state, connectivity state, and current activity status from this store. If the store gets out of sync with backend reality, the UI can look correct while the backend is doing something else.

## 5. Generated Wails bindings

Files:

- `frontend/wailsjs/go/app/App.d.ts`
- `frontend/wailsjs/go/app/App.js`
- `frontend/wailsjs/go/models.ts`

These files expose Go methods to TypeScript as async functions.

Example:

```ts
import { StartTracking } from "../wailsjs/go/app/App";
```

Important rule:

- do not edit these files by hand

If the Go-side exported method or model changes, regenerate through the Wails workflow.

## 6. Global hooks

### `useOnlineStatus`

This hook listens to browser `online` / `offline` events and updates Zustand.

Important limitation:

- browser online state only means the WebView thinks network connectivity changed
- it does not prove the FES API is reachable

### `useFirebaseAuth`

This hook subscribes to Firebase auth state.

Important reality:

- the app does initialize Firebase
- but the main working login path is not a Firebase sign-in flow
- the real session is mostly driven by Wails login calls and local store state

This hook is therefore contextual, but it is not the core source of truth for a working tracker session.

## 7. Layouts

### `MainLayout.tsx`

Used for the main task-tracking screens:

- full-height shell
- scrollable content
- bottom `ProfileBar`

### `SettingsLayout.tsx`

Used for settings-like screens:

- sticky header with back button
- central content column

## 8. Page responsibilities

### `Landing.tsx`

- temporary splash screen
- redirects to `/login`

### `Login.tsx`

Main online login screen.

Responsibilities:

- collect email and password
- call `LoginUserOnline`
- create frontend user session state
- store user in `localStorage`
- route to `/home`

Notable behavior:

- it sets user state from the returned attendance object first
- detailed user profile data is fetched later on the home screen

### `OfflineLogin.tsx`

Offline local login screen.

Responsibilities:

- fetch locally known accounts from Go
- let the user select an email
- call `LoginUser(email)`
- store local-only user state

### `Home.tsx`

This is the main operational dashboard.

Responsibilities:

- fetch tasks
- fetch current active task record
- refresh recent task records
- fetch and display today summaries
- listen for backend sync events
- show sync health and task list ordering

Important event subscriptions:

- `employee_details_fetched`
- `record-synced`
- `attendance-synced`
- `tasks-sync-warning`

### `Task.tsx`

This is the most important user-flow page.

Responsibilities:

- start and stop time tracking
- display current or latest session duration
- edit memo and activity tag
- show sync status
- display latest screenshot if enabled
- launch Upwork helper behavior for certain tasks

This page is the easiest place to trace a full end-to-end feature because it touches:

- Wails bindings
- Zustand
- runtime events
- sync state
- optional screenshot behavior

### `ActivityLogs.tsx`

This is the support and recovery screen for operational history.

Responsibilities:

- query activity logs for a selected date
- query per-client summary totals
- show attendance and task sync state
- permit manual resync actions
- enforce a 48-hour resync window for attendance login/logout entries

This page is especially important when debugging real-world sync failures.

### `AccountSettings.tsx`

Responsibilities:

- show user profile data
- manage light/dark theme
- show screenshot storage usage
- clear screenshot storage

### `Notifications.tsx`

Currently a UI-only settings screen. It does not appear to persist settings to a backend subsystem yet.

### `Logout.tsx`

Implements a guarded logout flow with best-effort backend logout and client cleanup.

## 9. Runtime event architecture

The frontend uses `EventsOn(...)` to subscribe to backend-emitted Wails events.

This is how the UI learns about:

- sync completion
- employee-detail refresh
- screenshot capture completion
- warning conditions

This means some state changes happen outside the immediate promise that started the action. When debugging stale UI, always inspect both:

- the original method call
- the matching runtime event handling

## 10. Local browser storage

Current frontend local storage usage includes:

- `user`
- `theme`
- `memo`
- `tag`
- `authToken` cleanup in logout paths

Practical notes:

- `theme` is preserved across logout
- some fields are read opportunistically rather than through a single storage abstraction

## 11. Styling system

The UI uses:

- Tailwind utility classes
- component primitives in `frontend/src/components/ui`
- custom components in `frontend/src/components/custom`
- some SCSS imports in `App.tsx`

The styling model is incremental rather than strongly design-system-driven. Keep visual changes consistent with existing page patterns unless you are intentionally redesigning a screen.

## 12. Timezone handling

Time and date rendering are especially important because the app tracks work sessions.

Relevant files:

- `frontend/src/lib/timezone.ts`
- `frontend/src/lib/utils.ts`

The frontend tries to normalize date-only behavior to the Manila timezone (`Asia/Manila`) and has a fallback strategy if that timezone is not supported by the environment.

If you touch logs, summaries, or date filters, read both frontend and backend timezone helpers together.

## 13. Frontend caveats and implementation mismatches

### Screenshot flag mismatch

Frontend release flags say screenshots are enabled, but the backend capture and Backblaze upload constants are disabled.

### Store naming mismatch

`AuthContext.ts` is a Zustand store, not a React context. Keep that in mind when searching or refactoring.

### Auth-source ambiguity

Firebase auth state exists, but the active product flow uses Wails backend login methods and local state. Avoid assuming the Firebase hook alone describes session truth.

### Login hydration is multi-step

The login page establishes a minimal user state first. Full employee details are fetched later by the home screen. This can affect assumptions about `user.name`, `user.position`, and other profile fields immediately after login.

### Generated types are part of the contract

If a backend model changes shape, the frontend type assumptions will change after regeneration. Do not edit the generated TypeScript to "fix" a mismatch.

## 14. Frontend testing situation

There are no visible frontend test files in the current repository structure. Most verification is currently manual plus backend tests.

That means frontend changes should be manually checked through:

- login flow
- task start/stop
- activity log loading
- sync/resync UI
- theme switching
- offline behavior

## 15. Good first tracing exercises

If you are learning the frontend, trace one of these:

### Start tracking flow

`Task.tsx` -> `StartTracking` -> backend -> `record-synced` event -> UI refresh

### Home dashboard refresh

`Home.tsx` -> `GetTasks` / `GetLastTaskRecords` / `GetActiveTaskRecord` -> sorted task rendering

### Activity resync flow

`ActivityLogs.tsx` -> confirmation dialog -> `SyncUnsyncedTaskRecord` or `SyncAttendanceByID` -> reload page state

## 16. Read next

- `guide/backend.md` for the Go implementation behind these calls
- `guide/data-and-sync.md` for the storage and lifecycle model
