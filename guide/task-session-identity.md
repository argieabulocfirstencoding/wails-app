# Task session identity: Bubble contract and rollout

This desktop release creates a UUID `client_session_id` with every newly started local task record. The value is durable in SQLite and identifies one task session, independent of retries, restarts, or browser reconciliation.

## Required Bubble `wf/task_in` behavior

The endpoint must accept `client_session_id` for the authenticated employee. When the field is present, it must perform one atomic create-or-return operation keyed by:

```
(authenticated employee identity, client_session_id)
```

The endpoint must never use a request-supplied employee identifier as the authority for this lookup. The authenticated Bubble user (or its verified employee record) is the owner.

For a new key, create one Productivity record, store the session ID, and return:

```json
{ "productivity": "<Bubble productivity unique id>", "client_session_id": "<submitted UUID>" }
```

For a repeated request using the same authenticated employee and key, return the original Productivity ID and session ID without creating another record or changing the original task-in fields. The unique constraint and duplicate handling must be inside one database transaction or equivalent atomic Bubble workflow; a search followed by a separate create is insufficient.

If the same key is presented by another authenticated employee, reject it. If it is associated with incompatible immutable fields (`task_in`, task, or attendance/login identity), reject it and record an operational error. Do not silently create a second Productivity record.

`wf/task_out` must remain idempotent for a Productivity ID. Repeated delivery should preserve the first accepted end state, or only accept a demonstrably identical repeat.

## Required browser-sync response

Each Productivity item returned to the desktop must include the original `client_session_id` when available:

```json
{
  "_id": "<Bubble productivity unique id>",
  "client_session_id": "<submitted UUID>",
  "task_in": 0,
  "task_out": 0,
  "projects_projects": "<task id>"
}
```

The desktop first reconciles by Bubble Productivity ID, then by the employee-scoped session ID. It deliberately does not auto-merge timestamp-only matches; those are marked for review to avoid joining two separate sessions.

## Rollout and staging gate

The desktop sends `client_session_id` to `wf/task_in` only when `FES_TASK_SESSION_IDEMPOTENCY_ENABLED=true`. The default is disabled. With the flag off, a request whose response is lost is marked as uncertain and cannot be automatically replayed, which prevents an unsafe duplicate.

Enable the flag only after all staging checks pass:

1. Submit the same authenticated employee/key concurrently at least ten times; confirm one Productivity exists and every response returns its same ID.
2. Repeat the key after deliberately dropping the first response; confirm the retry returns the original ID and does not create a second record.
3. Submit the same key as a different employee; confirm it is rejected and neither employee gains an extra record.
4. Send an incompatible repeated task-in payload; confirm it is rejected and the original record is unchanged.
5. Deliver `wf/task_out` twice and confirm the stored end state remains correct.
6. Fetch browser records and confirm `client_session_id` is returned exactly for the record created by `wf/task_in`.
7. Run the desktop staging scenarios: response loss followed by restart, Stop while task-in is delayed, browser reconciliation before and after Stop, and a timestamp-only collision. Confirm there is one remote Productivity and no local automatic merge for the collision.

Record the Bubble workflow version, staging evidence, and verifier before enabling the flag in production. Leave the flag unset or `false` on all other deployments.
