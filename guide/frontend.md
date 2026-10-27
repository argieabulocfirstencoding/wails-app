# Frontend Deep Dive

This document explains the React side of the app.

Even though the frontend is written with normal web tooling, remember that it is running inside a desktop WebView and depends heavily on Wails bindings and backend runtime events.

## 1. Frontend stack in plain language

The frontend is built with:

- React
- TypeScript
- Vite
- React Router
- Zustand
- Tailwind CSS
- Radix UI primitives
- some SCSS
- Firebase SDK

What these mean in simple terms:

- React: builds screens from components
- TypeScript: adds types to JavaScript
- Vite: development server and build tool
- React Router: page navigation inside the app
- Zustand: shared in-memory state store
- Tailwind: utility-first styling
- Radix UI: accessible UI primitives
- Firebase SDK: auth listener integration still present in the app

## 2. Frontend entry points

### `frontend/src/main.tsx`

This is the browser-side bootstrap file.

It:

- creates the React root
- mounts the app
- wraps the app with shared providers and safety layers

### `frontend/src/App.tsx`

This is the main application shell.

It:

- activates `useFirebaseAuth()`
- activates `useOnlineStatus()`
- sets up the router
- listens for update download progress; users manually check for updates from Account settings

### Why `App.tsx` matters

If the app has a global behavior problem, start here. This file is where the frontend's always-on hooks are installed.

## 3. Routing structure

Current routes include:

| Route | Main file | Purpose |
| --- | --- | --- |
| `/` | `pages/Landing.tsx` | splash/redirect behavior |
| `/login` | `pages/Auth/Login.tsx` | normal online login |
| `/offline-login` | `pages/Auth/OfflineLogin.tsx` | local offline login |
| `/home` | `pages/Main/Home.tsx` | main task list and current-day dashboard |
| `/task/:id` | `pages/Main/Task.tsx` | single-task tracking screen |
| `/account` | `pages/Account/AccountSettings.tsx` | account/settings screen |
| `/notifications` | `pages/Main/Notifications.tsx` | notifications UI |
| `/activity-logs` | `pages/Main/ActivityLogs.tsx` | history, sync state, and resync screen |
| `/logout` | `pages/Auth/Logout.tsx` | logout flow |

Unknown routes currently fall back to the login screen.

## 4. Layouts

Main layout files:

- `frontend/src/layouts/MainLayout.tsx`
- `frontend/src/layouts/SettingsLayout.tsx`

These give the app a shared shell for certain page groups.

### `MainLayout`

Used for the core tracking experience, such as Home and Task pages.

### `SettingsLayout`

Used for settings-style screens with simpler content flow.

## 5. Shared state: the Zustand store

Main file:

- `frontend/src/store/AuthContext.ts`

Important naming warning:

- despite the filename, this is not a React Context implementation
- it is a Zustand store

### Important state fields

- `user`
- `attendanceSynced`
- `loading`
- `online`
- `taskID`
- `fetchedActiveRecord`
- `synced`
- `tasks`
- `summaries`
- `attendance`
- `initializedTask`

### Why this store matters

Most screens rely on it to decide:

- whether the user appears logged in
- whether the app appears online
- which task is running
- which tasks to show
- what sync state badge to display

If the store is stale, the UI can look correct while the backend is in a different state.

## 6. Generated Wails bindings

The frontend imports backend-callable functions from generated files under `frontend/wailsjs/`.

Examples:

- `frontend/wailsjs/go/app/App.js`
- `frontend/wailsjs/go/app/App.d.ts`
- `frontend/wailsjs/go/models.ts`

These files let frontend code do this:

```ts
import { StartTracking, GetTasks } from "../wailsjs/go/app/App";
```

### Important rule

Do not edit generated binding files by hand.

If an `App` method changes or a Go model shape changes:

- regenerate the bindings through the Wails workflow

## 7. Global hooks

### `useOnlineStatus`

File:

- `frontend/src/hooks/useOnlineStatus.ts`

This hook listens to browser `online` and `offline` events and updates the Zustand store.

Important caveat:

- browser online status only tells you the WebView thinks the network changed
- it does not prove the FES API is reachable

### `useFirebaseAuth`

File:

- `frontend/src/hooks/useFirebaseAuth.ts`

This hook subscribes to Firebase auth state and writes the result into the same `user` field in the Zustand store.

Important caveat:

- the main working login flow of the app is driven by Wails + FES API calls
- Firebase auth state still mutates the same frontend store field

That means the codebase has some auth-source ambiguity. Be careful when changing user hydration or session state logic.

## 8. Release flags

Main file:

- `frontend/src/config/release.ts`

Current flags include:

- `screenshotsEnabled: true`

### Why this matters

The frontend sometimes conditionally renders screenshot UI based on this file. If you change screenshot behavior, align this with backend constants and actual product intent.

## 9. Important pages

### `Landing.tsx`

This is the temporary initial page. Think of it as a splash or entry redirect.

### `Auth/Login.tsx`

This is the normal online login screen.

It is responsible for:

- collecting credentials
- calling `LoginUserOnline`
- initializing frontend session state
- storing user information locally
- navigating to `/home`

### `Auth/OfflineLogin.tsx`

This screen supports local-only login using previously known accounts.

It is responsible for:

- fetching locally known accounts
- allowing account selection
- calling offline login
- continuing without remote validation

### `Main/Home.tsx`

This is one of the most important screens.

It is responsible for:

- fetching tasks
- fetching the active task record
- fetching current attendance
- listening for sync events
- showing recent task usage
- showing summary totals
- refreshing task ordering

Important runtime event subscriptions here:

- `employee_details_fetched`
- `record-synced`
- `attendance-synced`
- `tasks-sync-warning`

### `Main/Task.tsx`

This is the best page to study if you want to understand the full app.

It is responsible for:

- start and stop tracking
- showing timer state
- editing memo and activity tag
- showing sync state
- showing latest screenshot
- triggering Upwork helper behavior for qualifying tasks

Important runtime event subscriptions here:

- `screenshot`
- `record-synced`
- `upwork-button`

### `Main/ActivityLogs.tsx`

This is the support and recovery screen.

It is where a maintainer or support user is most likely to inspect:

- attendance sync state
- task-record sync state
- manual resync actions
- recent work history

### `Account/AccountSettings.tsx`

This page includes user-facing maintenance actions like checking screenshot storage usage and clearing screenshots.

### `Auth/Logout.tsx`

This page drives logout and cleanup behavior. Because logout touches attendance sync, it is more than a simple client-side state reset.

## 10. Home page behavior in more detail

`frontend/src/pages/Main/Home.tsx` is a good example of the frontend's hybrid model.

It does all of these things:

- fetches user details
- fetches task summaries
- fetches recent task records
- fetches current active record
- listens for sync events
- re-sorts tasks based on activity
- displays sync warnings from the backend

### Interesting detail

The page does not just "load once". It is refreshed by backend-emitted events. That means a frontend-only reading of the file is not enough; you also have to know what backend events exist.

## 11. Task page behavior in more detail

`frontend/src/pages/Main/Task.tsx` is where the user experiences the tracking lifecycle most directly.

### When starting a task

The page:

- calls `StartTracking`
- stores the start time locally for display
- updates UI state to show the task as running
- may set the `padagan` mode

### When stopping a task

The page:

- calls `StopTracking`
- clears active task state
- shows sync state changes as the backend finishes background work

### Memo and tag behavior

The page stores memo and tag values locally and sends them to the backend through:

- `AddMemo(...)`
- `SetTaskRecordNotes(...)`

### Screenshot behavior

If screenshots are enabled and runtime events arrive, the page updates the latest screenshot preview without waiting for a full page reload.

## 12. Event-driven UI architecture

The frontend uses Wails runtime event listeners with `EventsOn(...)`.

This matters because not all state changes arrive in the original command response.

Typical pattern:

1. frontend calls a backend method
2. backend starts work or queue work
3. backend later emits an event
4. frontend listener updates UI

### Consequence for debugging

When a page looks stale, check:

- whether the initial method call worked
- whether the expected event was emitted
- whether the page subscribed correctly
- whether the event handler refreshed the right local state

## 13. Local browser storage

The frontend uses `localStorage` for small pieces of persistent client-side state.

Examples include:

- `user`
- `theme`
- `memo`
- `tag`

### Important note

This storage is useful for UX, but it is not the durable operational source of truth for time tracking. The SQLite database on the backend still matters more.

## 14. Styling system

The UI uses a mix of:

- Tailwind utility classes
- reusable UI primitives in `components/ui`
- custom components in `components/custom`
- SCSS imports from `App.scss` and `styles/style.scss`

This means the styling system is practical and mixed, not a rigid design system. Preserve consistency with current patterns unless you are doing an intentional redesign.

## 15. Time and timezone behavior

Time handling is important because the app is a work tracker.

Relevant frontend files include:

- `frontend/src/lib/timezone.ts`
- `frontend/src/lib/utils.ts`

The app tries to keep date behavior aligned to Manila time. If you change date rendering, filters, logs, or summaries, treat timezone changes carefully.

## 16. Frontend caveats a new developer should know

### Auth state is slightly ambiguous

The same store field can be touched by:

- Firebase auth listener behavior
- Go-backed login flow

That deserves care during refactors.

### `AuthContext.ts` is not React Context

This sounds small, but it can confuse search and mental models.

### Generated bindings are not source code you maintain manually

If something looks wrong there, fix the Go side or rerun Wails generation.

### Browser online status is not full connectivity truth

The WebView may think it is online even if the FES API is unavailable.

### Screenshot UI depends on backend behavior

You can render screenshot placeholders in the frontend, but the actual capture and upload flow is backend-owned.

## 17. Good frontend reading order

If you are learning the frontend, this order works well:

1. `frontend/src/App.tsx`
2. `frontend/src/store/AuthContext.ts`
3. `frontend/src/pages/Main/Home.tsx`
4. `frontend/src/pages/Main/Task.tsx`
5. `frontend/src/pages/Main/ActivityLogs.tsx`
6. `frontend/src/pages/Auth/Login.tsx`
7. `frontend/src/hooks/useOnlineStatus.ts`
8. `frontend/src/hooks/useFirebaseAuth.ts`

## 18. Good change patterns

### If you need to add a new button that triggers backend work

1. add the button in the page or component
2. import the generated Wails binding
3. call the binding
4. update local UI state for immediate feedback
5. if backend work is asynchronous, subscribe to or reuse a runtime event as needed

### If you need to show new backend data

1. confirm the data exists on the Go side
2. confirm it is exposed through a Wails method or model
3. regenerate bindings if shape changed
4. update the consuming page or store

### If you need to adjust task or sync UX

Read the matching backend flow first. Much of the truth lives there.

## 19. Manual testing guidance for frontend work

There are no visible frontend test files in the repo right now, so manual testing matters.

After frontend changes, manually check at least:

1. login flow
2. offline login flow if your change might affect session state
3. home page task loading
4. task start and stop
5. task memo/tag editing
6. activity logs and resync UI
7. account settings if screenshots or storage are involved
8. logout flow

## Read next

- [`guide/backend.md`](./backend.md) to understand the Go methods this UI calls
- [`guide/data-and-sync.md`](./data-and-sync.md) for the record lifecycle behind the screens
