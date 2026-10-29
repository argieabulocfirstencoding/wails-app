# Bubble workflows: conflict resolution

Publish two authenticated Bubble API workflows before enabling user conflict resolution in a release. Both use `POST`, Bubble's `User & admin` authentication, and JSON Object responses. The desktop sends RFC3339 UTC dates so native Bubble workflow parameters can be typed as `date`.

## Attendance: `wf/resolve_attendance_conflict`

```json
{
  "record_id": "Bubble thing ID",
  "expected_modified_at": "2026-10-30T08:00:00Z",
  "time_in": "2026-10-30T08:00:00Z",
  "time_out": null
}
```

The workflow updates the `Productivity - Login Logout` record's `Login` and `Logout` fields.

## Task: `wf/resolve_task_conflict`

```json
{
  "record_id": "Bubble thing ID",
  "expected_modified_at": "2026-10-30T08:00:00Z",
  "started_at": "2026-10-30T08:00:00Z",
  "ended_at": "2026-10-30T08:15:00Z",
  "duration": 900,
  "notes": "optional note",
  "padagan": false
}
```

The workflow updates the `Productivity - Productivity` record's `Actual Task In`, `Actual Task Out`, `Total Task Time`, `ActivityDescription`, and `Operate from TopTracker (Padagan)` fields. The last field stores Bubble's existing `yes`/`no` representation.

Each workflow must authenticate the employee, verify ownership of the requested Bubble thing, and compare its current Modified Date against `expected_modified_at` before writing. It must update only the listed mutable fields. If the version changed, return a successful API response containing `{ "stale": true }` and make no write. Otherwise return `{ "stale": false, "modified_at": "<current Modified Date as RFC3339>" }` after the update.

The desktop never resolves remote IDs, task/attendance links, or employee ownership. Those remain blocked identity conflicts.
