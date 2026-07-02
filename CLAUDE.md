# krci-audit

`github.com/KubeRocketCI/krci-audit` — a **self-contained, platform-agnostic** Kubernetes
admission audit service: **kube-audit-rest (capture) + Vector (ship) + its own PostgreSQL
(store)**. It records a standard, identity-attributed, append-only event per Kubernetes
mutation, without control-plane access and without ever blocking platform operations.

This repo is the **capture + store foundation**. A read/export API, scheduled
partition-rotation, and downstream consumers (Portal, SIEM) are layered on top separately.

## Build & Test

```
make build        # → dist/krci-audit-migrate-<arch>  (CGO_ENABLED=0)
make test         # go test ./... -coverprofile=coverage.out
make test-unit    # fast unit tests only (no Docker)
make lint         # golangci-lint v2
make helm-lint    # helm lint deploy-templates
make helm-template
```

- **Unit tests** (`internal/config`, `internal/dsn`, `internal/models`, `pkg/identity`) run anywhere.
- **Store integration tests** (`internal/store`) start a throwaway PostgreSQL via
  **testcontainers** → require **Docker**. They apply the real embedded migrations and assert
  dedup, append-only, dry-run exclusion, partition pruning, and partition-drop retention. If
  Docker is unavailable they **skip** (not fail).
- **Helm render tests** (`test/deploy`) shell out to **helm** and assert the webhook /
  Vector config guarantees. If `helm` is absent they **skip**.

Run a single package: `go test ./internal/store/... -run TestDedup`.

## Architecture

### Capture → ship → store

1. **kube-audit-rest** (`ValidatingWebhookConfiguration`, `failurePolicy: Ignore`,
   `timeoutSeconds: 1`) logs the raw `AdmissionReview` (`admission.k8s.io/v1`) + an injected
   `requestReceivedTimestamp` to a shared `/tmp` volume. It always returns `allowed: true`.
2. **Vector** (sidecar, shares the pod uid so it can read the 0600 log) tails the log →
   `parse` → `filter` (which events are stored; default = KRCI objects) → `shape` (reshape to
   the `audit_events` columns; operation-aware name/object_uid; capture level) → `postgres`
   sink with `jsonb_populate_record` and `batch.max_events: 1`.
3. **PostgreSQL** `audit_events`: one row per admission event, RANGE-partitioned monthly,
   composite PK `(event_uid, received_at)`, `BEFORE INSERT` dedup trigger, least-privilege
   `audit_writer` (INSERT/SELECT only) → append-only.

### Layout

```
cmd/krci-audit-migrate/   — migration runner CLI (thin main → config.Load + migrate.RunCLI)
internal/config/          — typed runtime config (env → Config)
internal/models/          — domain types: AuditEvent + Operation/CaptureLevel enums + column source of truth
internal/migrate/         — golang-migrate wrapper over embedded SQL + RunCLI + SetWriterPassword
internal/dsn/             — DSN resolution (full DSN or discrete PG* env → pgx5)
internal/store/           — schema identifier constants (table/view/roles) + store integration tests
pkg/identity/             — reusable actor classification (human vs automation vs unknown)
migrations/               — versioned SQL (embedded); the store DDL is HIGH-risk (freezes the PK/schema)
deploy-templates/         — Helm chart (webhook, Vector ConfigMap, Deployment, migration Job, cert, DB provisioning)
test/deploy/              — Helm chart render tests
```

**Layering.** `internal/models` is the single home for the audit vocabulary — column names
derive from `AuditEvent` struct tags (`AllColumns`/`LiftedColumns`), and a store integration
test asserts they match the live table, so a migration change can never silently desync them.
Reusable, dependency-free helpers live under `pkg/`. The store exposes only schema
**identifiers** (`EventsTable`, `RealView`, `WriterRole`, `ReaderRole`).

**A read/export API drops in cleanly** — add `internal/api/` (OpenAPI spec + generated
server/handlers), generate response DTOs mapped from `models.AuditEvent`, a
`cmd/krci-audit-api/` binary, a `make generate` target, and a chart API Service/Deployment.
The read path connects as the least-privilege **`audit_reader`** role (SELECT-only; created by
migration `000003`, distinct from the ingestion `audit_writer`), so the hooks already exist.

### Database provisioning (`db.mode`)

The chart supplies PostgreSQL in one of three selectable modes:

| `db.mode` | Provisions | Owner creds | Extra requirements |
|---|---|---|---|
| `external` (default) | nothing — bring your own DB | `db.owner.secretName` (keys `user`/`password`) | set `db.host` |
| `pgo` | a Crunchydata `PostgresCluster` | operator Secret `<release>-pguser-<release>` | postgres-operator add-on installed |
| `simple` | a single plain Postgres `Deployment` + `Service` (+PVC) | chart Secret `<release>-db` | none (dev/small) |

Append-only is identical across modes: the migration Job (schema owner) applies the schema
and sets the LOGIN password of the least-privilege `audit_writer` role from a chart-managed
writer Secret (`db.writer`); Vector always connects as `audit_writer`, never the owner. The
migrator builds its DSN from `AUDIT_DB_DSN` or discrete `PG*` env (`internal/dsn`).

### Data model

Column names mirror `AdmissionReview` payload paths so the Vector `postgres` sink maps
directly (no surrogate `id`). `event_uid` (= `request.uid`) is the logical event identity +
dedup key; `object_uid` (= object/oldObject `metadata.uid`) is the correlation column. Query
surface = the lifted typed columns only (v1); `object`/`old_object`/`raw` are stored and
retrievable but not searchable (no JSON/GIN path — deferred).

## Conventions / Do NOT use

- **No surrogate `id` column** — the Vector sink writes every column from the event.
- **No native control-plane audit, no Tekton Results, no OpenSearch** — this component owns
  its own Postgres and depends on no other system.
- **Never hand-edit generated code.** (None yet; a read API would add oapi-codegen output.)
- **Retention = drop/detach whole partitions**, never row-by-row `DELETE`.
- Credentials/DSNs come from a Secret provisioned by the deployment, never committed.
