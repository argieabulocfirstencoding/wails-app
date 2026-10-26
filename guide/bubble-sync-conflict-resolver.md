# Bubble workflow: resolve_sync_conflict

Publish an authenticated Bubble API workflow at `wf/resolve_sync_conflict` before enabling user conflict resolution in a release.

The desktop sends:

```json
{
  "record_type": "attendance | task",
  "record_id": "Bubble thing ID",
  "expected_modified_at": 1761264000000,
  "values": {
    "time_in": 1761264000000,
    "time_out": null
  }
}
```

For `task`, `values` contains `started_at`, `ended_at`, `duration`, `notes`, and `padagan`. For `attendance`, it contains `time_in` and `time_out`.

The workflow must authenticate the employee, verify ownership of the requested Bubble thing, and compare its current Modified Date against `expected_modified_at` before writing. It must update only the listed mutable fields. If the version changed, return a successful API response containing `{ "stale": true }` and make no write. Otherwise return `{ "stale": false, "modified_at": <new epoch milliseconds> }` after the update.

The desktop never resolves remote IDs, task/attendance links, or employee ownership. Those remain blocked identity conflicts.
