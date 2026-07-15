# Browser Task-In Event Contract

This contract connects Bubble's browser task-in workflow to the installed
desktop tracker. Bubble never calls an employee computer directly; the desktop
app reads events from Bubble after the employee logs in.

## 1. Bubble producer: `task_in_browser`

Run this backend workflow only after Bubble has created the Productivity
record. Its single input is:

```json
{
  "prod_id": "Bubble Productivity unique ID"
}
```

The workflow must create one unacknowledged desktop-task event for the
Productivity record's employee. Do not create a second pending event for the
same `prod_id`.

Recommended event fields in Bubble:

- `prod_id` (text, required)
- `employee` or employee unique ID (required, used to scope the event)
- `status` (`pending` or `acknowledged`)
- `created_at`
- `acknowledged_at`

## 2. Desktop pull endpoint: `wf/pending_browser_task_events`

The desktop calls this endpoint every 15 seconds while authenticated and
online. The desktop feature is intentionally off by default; enable it only
after these two Bubble endpoints are published by launching the app with
`FES_BROWSER_TASK_EVENTS_ENABLED=true`. Identify the employee from the bearer
token; do not trust an employee ID supplied by the desktop.

Return only unacknowledged events for that employee:

```json
{
  "events": [
    {
      "event_id": "optional Bubble event ID",
      "prod_id": "Bubble Productivity unique ID"
    }
  ]
}
```

`prod_id` is required. The desktop also accepts `productivity_id` as a
backward-compatible alias. An empty list means no task needs action.

## 3. Desktop acknowledgement endpoint: `wf/acknowledge_browser_task_event`

After the desktop has imported the Productivity record and activated it (when
the record is still active), it sends:

```json
{
  "event_id": "optional Bubble event ID",
  "prod_id": "Bubble Productivity unique ID"
}
```

Mark the event acknowledged for the authenticated employee. This operation
must be idempotent: repeating the same acknowledgement must succeed without
creating another task-in.

## Desktop behavior

The desktop uses `prod_id` to reconcile the complete Productivity record via
its existing browser-record sync. It then adopts the already-imported record
as the active desktop task without creating a second remote task-in. If the
productivity record already has a task-out time, it is imported as history and
acknowledged without starting screenshots.
