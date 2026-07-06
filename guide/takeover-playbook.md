# Takeover Playbook

This document is a practical guide for a developer who is taking over the project and wants a clear path from "I barely know this app" to "I can safely change and support it."

It is not a replacement for the deeper docs. It is the shortest reliable roadmap through them.

## 1. Your real job in week one

Your week-one goal is not to memorize everything.

Your week-one goal is to become safe.

In this project, "safe" means:

- you can run the app locally
- you know where core behavior lives
- you can trace a bug from UI to DB
- you know which areas are high-risk
- you can make a small change without breaking sync or local data

## 2. Day 1 plan

### Step 1: get the app running

- install dependencies
- run `wails dev`
- make sure the window opens

### Step 2: click through the core screens

Visit:

- Login
- Offline Login
- Home
- one Task page
- Activity Logs
- Account Settings

### Step 3: read the smallest set of core files

Read:

1. `main.go`
2. `backend/app/app.go`
3. `frontend/src/App.tsx`
4. `frontend/src/store/AuthContext.ts`

### Step 4: read the guide in this order

1. `guide/architecture.md`
2. `guide/backend.md`
3. `guide/frontend.md`
4. `guide/data-and-sync.md`

## 3. The first behavior you should trace

Trace "start a task" or "stop a task".

Why?

Because that flow touches:

- frontend page logic
- Wails bindings
- backend facade
- services
- repositories
- SQLite
- sync
- screenshots

If you understand that one workflow, you understand a big part of the app.

## 4. The one mental model that unlocks the repo

Think of the app as:

- a local-first tracker
- with a React face
- a Go brain
- a SQLite memory
- and a sync engine trying to keep remote systems caught up

If you instead think of it as "a website with a local DB", you will keep making wrong assumptions.

## 5. The questions to ask before any code change

1. Is this behavior frontend-only, backend-only, or cross-layer?
2. Does this need to work offline?
3. Does this touch local records that later need sync?
4. Does the UI depend on a runtime event for refresh?
5. Is there a migration or data-shape consequence?
6. Are screenshots involved, even indirectly?

## 6. The safest first contributions

If you want to build confidence without touching the highest-risk systems, start with:

- documentation improvements
- log-message clarity
- non-structural UI polish
- small bug fixes in isolated page behavior
- backend tests around current behavior

## 7. The riskiest systems

Treat these areas with extra care:

- `backend/service_classes/sync.service.go`
- `backend/db/migrations.go`
- `backend/service_classes/task.service.go`
- attendance/task dependency rules
- screenshot capture and upload integration
- auth/session state hydration

## 8. How to debug a user report without panicking

If a user says "my work did not sync":

1. ask what screen/action they used
2. inspect the local attendance row
3. inspect the local task record row
4. inspect screenshot rows if relevant
5. inspect logs
6. check whether the issue is:
   - local record creation
   - remote sync failure
   - missing attendance dependency
   - stale frontend state

The app often preserves enough local state to explain what happened.

## 9. How to read code when you do not know Go well

You do not need to understand every Go feature to be effective here.

Use this method:

1. find the method name called by the frontend
2. read the function body top-to-bottom
3. list the other functions it calls
4. open those files
5. keep following the chain until you hit DB writes or API calls

Focus on control flow and data movement before trying to "master the language."

## 10. How to read code when you do not know Wails well

You mainly need to understand three Wails concepts:

1. Go methods on `App` can be called from the frontend
2. Wails generates bridge files for that contract
3. backend code can emit runtime events that the frontend listens to

That is enough to become productive.

## 11. Signs you are probably changing the wrong layer

- you are trying to solve a sync bug only in React
- you are trying to solve a UI stale-state bug only in Go without checking events
- you are changing DB columns but not migration logic
- you are changing exposed Go data without regenerating Wails bindings
- you are changing screenshot UI without checking capture/upload behavior

## 12. What "good enough understanding" looks like

You are ready to make normal changes when you can answer:

1. Where does login really happen?
2. What starts and stops a task?
3. Where is the active task stored?
4. Why can a record be local but not synced?
5. Why does attendance have to sync before a task?
6. How does the frontend hear about background sync completion?

## 13. First-week checklist

By the end of week one, aim to have done all of this:

1. run the app locally
2. read the main architecture docs
3. inspect at least one local database file
4. trace one full task workflow
5. run backend tests
6. make one low-risk change
7. understand where logs and screenshots are stored
8. know how to inspect sync state

## 14. Handover questions worth asking a previous maintainer

If a previous maintainer is available, these are high-value questions:

1. What production issue happens most often?
2. Which flows are known to be fragile?
3. Which accounts or environments are safe for testing?
4. Are screenshots expected to be fully active in production?
5. What release step is easiest to forget?
6. Which "legacy-looking" files are still definitely in use?

## Next step

After this playbook, read [`guide/troubleshooting.md`](./troubleshooting.md). That is the most useful companion when you start handling real issues.
