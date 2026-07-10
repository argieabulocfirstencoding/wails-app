**Clarified Goal**

Replace the current DB startup path with a single `gormigrate`-driven flow so [backend/db/db.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/db.go:26>) only resolves the DB path, opens SQLite, applies connection settings, runs ordered migrations, and returns. No bootstrap `AutoMigrate`, no ad hoc repair passes, and no custom version runner.

**Current Situation**

- Startup is currently doing five jobs at once in [backend/db/db.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/db.go:26>): open DB, decide backup, bootstrap tables, run repair helpers, then run a homegrown migration engine.
- Schema authority is split across runtime models, duplicate bootstrap structs in [backend/db/bootstrap.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/bootstrap.go:9>), and handwritten rebuild SQL in [backend/db/task_records_repair.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/task_records_repair.go:11>) and [backend/db/task_tables_repair.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/task_tables_repair.go:25>).
- Migration tracking is custom in [backend/db/migrations.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/migrations.go:11>) with `latestSchemaVersion = 3`, but a lot of real schema behavior still happens outside that version table.
- `clients` is used by app code and repositories ([backend/repository/task.repo.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/repository/task.repo.go:46>), [backend/services/task.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/services/task.go:66>)) but is not part of the current init bootstrap.
- The existing DB tests pass now, including `go test ./backend/db -count=1` and `go test ./... -count=1`, so the refactor should preserve current data-salvage behavior while simplifying the architecture.

**Implementation Plan**

1. Define the canonical schema first.
- Freeze the exact target schema for `users`, `clients`, `tasks`, `attendances`, `task_records`, and `screenshots`, including defaults, nullability, indexes, and foreign-key actions.
- Treat migration-owned DDL as the source of truth, then update runtime model tags later if they differ. Do not let runtime `AutoMigrate` define production schema anymore.

2. Redesign `db.Init` into a thin orchestrator.
- Keep DB path resolution and build-type file naming from [backend/db/db.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/db.go:26>).
- Move SQLite setup into explicit helpers: open connection, enable `foreign_keys`, apply `journal_mode=WAL`, apply `busy_timeout`, then call a single `runMigrations(db, dbPath)` function.
- Remove all direct calls to `bootstrapCoreSchema`, `bootstrapScreenshotSchema`, `repairTaskRecordsSchema`, `repairTaskAndScreenshotSchemas`, and `migrateSchema`.

3. Introduce a dedicated `gormigrate` runner.
- Add the GORM v2 `gormigrate` dependency.
- Create a migration catalog with ordered string IDs such as `20260701_001_users_canonical`.
- Use a new migration table name like `gorm_migrations`; do not reuse the current `schema_migrations` table for the first rollout.
- Wrap each migration in local helper metadata:
  - `ID`
  - `Description`
  - `RequiresBackup`
  - `Up`
  - `Down` or explicit rollback-impossible behavior for rebuild migrations

4. Use one migration path for both fresh and legacy DBs.
- Recommended: do not use a separate `InitSchema` path in the first refactor.
- Every migration should be conditional:
  - table missing -> create canonical table
  - table exists but legacy/unhealthy -> rebuild/copy into canonical table
  - table already canonical -> no-op
- This avoids having one schema path for fresh installs and another for upgrades.

5. Convert the current repairs into named baseline migrations.
- `users`: replace `repairUsersSchema` with a migration that creates `users` if missing, or rebuilds when `id` is numeric / `birthday` must become `birthdate`.
- `clients`: add an explicit canonical migration for `clients`; this is currently missing from init.
- `tasks`: migrate/rebuild `tasks` into the canonical shape and correct client FK behavior.
- `attendances`: create or normalize `attendances` explicitly instead of relying on bootstrap `AutoMigrate`.
- `task_records`: migrate current repair logic into a migration that preserves the existing backfill rule from attendance `user_id`.
- `screenshots`: migrate current repair logic into a migration that preserves existing salvage rules and required columns.
- `indexes`: move [backend/db/indexes.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/indexes.go:5>) into a migration that only manages indexes.

6. Standardize SQLite rebuild mechanics.
- Keep a single helper for “connection-scoped foreign keys off, explicit transaction, rebuild legacy table, copy, validate, drop legacy”.
- Use raw SQL for rebuild migrations instead of `AutoMigrate` on runtime models.
- Prefer explicit create/copy SQL over inferred GORM behavior for SQLite table rewrites.

7. Replace the backup policy with migration-aware preflight.
- Delete the heuristic version logic in [backend/db/backup_policy.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/backup_policy.go:5>).
- Before running migrations, inspect pending migration metadata and create one backup only if a pending migration can rebuild/drop populated tables and the DB file is non-empty.
- On failure, return the backup path in the error exactly once from the migration runner.

8. Retire the old schema system in stages.
- Remove `latestSchemaVersion`, `schemaMigration`, `migrationStep`, and the current custom migration runner from [backend/db/migrations.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/migrations.go:11>).
- Delete the bootstrap structs in [backend/db/bootstrap.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/bootstrap.go:9>) after the new migrations cover fresh DB creation.
- Leave the old `schema_migrations` table untouched for the first release after cutover; clean it up only after the new path is proven in one follow-up release.

9. Rebuild the test strategy around migrations instead of `AutoMigrate`.
- Rewrite [backend/db/migrations_test.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/db/migrations_test.go:22>) to test `runMigrations` end-to-end, not bootstrap internals.
- Remove tests that only protect bootstrap behavior, such as the parity and “AutoMigrate does not touch X” cases, because those concerns should disappear.
- Add a shared test helper that opens SQLite, enables `foreign_keys`, runs the full `gormigrate` stack, and returns a ready test DB.
- Replace direct `AutoMigrate` setup in [backend/app/active_task_cache_test.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/app/active_task_cache_test.go:24>), [backend/service_classes/screenshot.service_test.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/service_classes/screenshot.service_test.go:26>), and [backend/tests/attendance_test.go](</C:/xampp/htdocs/FES-Projects/fes-app/wails-app/backend/tests/attendance_test.go:23>) with that helper.

10. Do rollout validation with three DB classes.
- Fresh empty DB: confirm all six tables, indexes, and FKs exist with no backup.
- Current healthy DB: confirm migration is idempotent and skips backup.
- Crafted legacy/corrupt DB fixtures: confirm user repair, task record backfill, corrupted FK repair, and failure-on-invalid-data rules still behave as they do today.

**Acceptance Checklist**

- `db.Init` no longer calls `AutoMigrate` or any standalone repair helper.
- Fresh DB creation happens entirely through `gormigrate`.
- `clients` exists on a fresh DB and task/client preload paths work.
- Existing healthy DBs migrate with no schema churn and a second run is a no-op.
- Legacy `users`, `task_records`, `tasks`, and `screenshots` cases are either repaired safely or fail with explicit data-preservation errors.
- All tests pass using migration-built test databases, not ad hoc `AutoMigrate`.

**Key Decisions To Confirm Before Coding**

- Canonical FK behavior: recommended to align `tasks.client_id` and `attendances.user_id` with intended runtime semantics, then codify those actions in migration DDL.
- Old `schema_migrations` handling: recommended to leave it read-only for the first `gormigrate` release and write new history into a separate `gorm_migrations` table.
- Fresh DB path: recommended to avoid `InitSchema` in the first pass and run the same conditional migration list from zero.


Yes. This is now a strong implementation plan, and I would approve the direction with five important changes before coding.

Confirmed decisions

These are correct:

Use a new gorm_migrations table and leave the existing schema_migrations table untouched for the first release.
Do not use InitSchema in the first cutover. Run one ordered, conditional migration catalog for both new and existing databases.
Keep all application schema creation and repair inside Gormigrate migrations.
Remove AutoMigrate from application startup and tests that currently use it as a shortcut.
Preserve legacy salvage rules rather than replacing them with blind model-driven migrations.
Add clients explicitly so new databases cannot hit no such table: clients.

Gormigrate is appropriate here because it gives you version tracking, ordered migrations, configurable migration-table names, optional transaction handling, and unknown-migration validation. It also supports migrations with no rollback where rollback is genuinely unsafe.

Required change 1: do not defer model-tag alignment

This line needs adjusting:

Treat migration-owned DDL as the source of truth, then update runtime model tags later if they differ.

Change it to:

Treat migration-owned DDL as the source of truth and update runtime model tags in the same refactor before release.

The runtime models no longer control schema creation, but they still control how GORM reads, writes, names fields, handles defaults, treats nullable values, and maps relationships. A mismatch between the migration DDL and runtime model tags can still create application bugs even without AutoMigrate.

Required change 2: audit every table before saying “all six tables”

Your list now contains:

users
clients
tasks
attendances
task_records
screenshots

But the earlier architecture also referenced projects. Before coding, do one complete schema inventory from:

Every runtime model
Every repository query
Every service query
Every raw SQL statement
Every foreign key relationship
Every bootstrap struct and repair helper

Do not let the migration plan accidentally solve the missing-clients problem while introducing a missing-projects or another hidden-table problem.

Your acceptance criterion should be:

Every table referenced by runtime code, repositories, tests, or foreign keys must be created and maintained by the Gormigrate catalog.

Required change 3: fix the SQLite rebuild strategy

This is the most important technical correction.

Your plan says:

connection-scoped foreign keys off, explicit transaction, rebuild legacy table, copy, validate, drop legacy

That is the right intent, but it cannot run inside a Gormigrate-wide transaction.

SQLite documents that:

PRAGMA foreign_keys = OFF

is a no-op inside an active transaction or savepoint. Gormigrate’s UseTransaction: true wraps its migration work in one transaction, so a rebuild migration cannot reliably disable foreign-key enforcement from inside that flow.

Also, SQLite’s recommended safe rebuild sequence is:

1. Disable foreign keys before the transaction.
2. Begin transaction.
3. Create new_<table>.
4. Copy and transform data.
5. Drop old table.
6. Rename new_<table> to the original table name.
7. Recreate indexes, triggers, and views.
8. Run foreign_key_check.
9. Commit.
10. Re-enable foreign keys.

SQLite specifically warns against the old pattern of renaming the original table first, such as:

ALTER TABLE users RENAME TO users_legacy;

because it can break references in foreign keys, triggers, and views. The safer pattern is to create new_users, copy into it, drop users, then rename new_users to users.

Recommended implementation rule

Use:

UseTransaction: false

for the Gormigrate runner.

Then:

Normal migrations may open their own GORM transaction.
Rebuild migrations use one dedicated SQLite connection.
That helper disables foreign keys before starting its explicit transaction.
The helper performs the full new_table → copy → validate → swap process.
The migration remains conditional, so if the app crashes after the schema is rebuilt but before Gormigrate records its ID, the next run sees the canonical table and safely no-ops before recording the migration.

That is still Gormigrate-driven. You are not creating another versioning engine; you are only creating a safe SQLite rebuild helper.

Required change 4: make backup decisions based on the planned action, not only migration metadata

This part needs refinement:

Create one backup only if a pending migration can rebuild/drop populated tables and the DB file is non-empty.

A newly opened SQLite database file can be non-empty even when it contains no application data. So file size is not a reliable signal for whether a backup is needed.

Use a migration preflight model instead:

type MigrationPlan struct {
    ID             string
    Description    string
    Pending        bool
    Action         string // no-op, create, alter, rebuild
    HasUserData    bool
    RequiresBackup bool
}

Then create a backup only when all of these are true:

Migration is pending
AND migration will actually rebuild or drop a table
AND the affected table contains rows

That ensures a healthy existing DB with an empty gorm_migrations table does not get a backup merely because all first-rollout migrations are technically pending but ultimately no-op.

Required change 5: make the backup WAL-safe

Because you are enabling:

PRAGMA journal_mode = WAL

do not rely on a simple file copy of only:

fesapp.db

In WAL mode, committed changes may live in the separate WAL file until checkpointing. SQLite recommends using its backup mechanism or another SQLite-aware snapshot method for a consistent live backup.

Your backup helper should use one of these approaches:

SQLite backup API, if supported cleanly by the chosen driver.
VACUUM INTO to create a snapshot file.
A tested checkpoint-and-copy implementation that safely accounts for the WAL sidecar files.

For this migration cutover, I would prefer an SQLite-aware snapshot rather than copying the database file manually.

Recommended Gormigrate configuration
options := &gormigrate.Options{
    TableName:                 "gorm_migrations",
    IDColumnName:              "id",
    IDColumnSize:              255,
    UseTransaction:            false,
    ValidateUnknownMigrations: true,
}

ValidateUnknownMigrations: true is a good safety measure. It prevents the application from silently accepting a database whose recorded Gormigrate IDs no longer exist in source control. Gormigrate also has no built-in lock, so keep startup single-instance or add an app-level process lock if the desktop app could run twice against the same database file.

Your migration wrapper is fine, with one boundary

This is acceptable:

type CatalogMigration struct {
    ID              string
    Description     string
    RequiresBackup  bool
    Plan            func(*gorm.DB) (MigrationPlan, error)
    Up              func(*gorm.DB) error
    Down            func(*gorm.DB) error
}

But enforce this rule:

The wrapper may inspect pending IDs, build a backup plan, and convert catalog entries into gormigrate.Migration values. It must never independently store migration versions, mark migrations complete, or decide migration order outside the catalog order.

Gormigrate must remain the sole writer of migration history.

For destructive rebuilds, Down should usually be nil. Do not invent rollback logic that could destroy data or incorrectly reverse an ID conversion. Gormigrate explicitly treats a missing rollback as rollback-impossible.

Canonical FK behaviour: confirm from application semantics, not convention

I would not lock this in until you inspect the existing models and delete flows.

A safe decision rule:

Relationship	Use this when	Typical action
tasks.client_id	A task can remain after its client is removed	ON DELETE SET NULL with nullable client_id
tasks.client_id	A task must never exist without a client	ON DELETE RESTRICT
attendances.user_id	Attendance history must remain protected	ON DELETE RESTRICT
attendances.user_id	Attendance should be deleted with the user	ON DELETE CASCADE

Do not use CASCADE merely because it is convenient. For attendance or work-history data, RESTRICT is usually the safer default unless the application deliberately supports hard-deleting users and their history.

Revised final verdict

Your clarified plan is correct once these changes are incorporated:

Align model tags with canonical DDL in the same release.
Complete a full schema inventory before fixing the table list.
Use a SQLite-safe new_table → copy → drop old → rename rebuild process.
Do not combine PRAGMA foreign_keys = OFF with gormigrate.UseTransaction = true.
Make backups based on actual planned destructive work, not only pending IDs or file size.
Make backups WAL-safe.
Leave old schema_migrations untouched and let gorm_migrations become the only active migration history table.

With those corrections, this is no longer “patch fixes converted to a library.” It becomes a proper, maintainable migration architecture with one authoritative schema path.