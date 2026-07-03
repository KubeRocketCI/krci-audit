# krci-audit

A self-contained, **platform-agnostic** Kubernetes admission audit service for
KubeRocketCI: **who changed what**, recorded from Kubernetes admission events, without
control-plane access and without ever blocking platform operations.

```txt
API server ──AdmissionReview──▶ kube-audit-rest (ValidatingWebhook, failurePolicy: Ignore)
                                     │  logs raw payload + requestReceivedTimestamp
                                     ▼
                                  Vector (sidecar): parse → filter → shape → postgres sink
                                     ▼
                                  PostgreSQL audit_events  (partitioned, append-only, deduped)
```

This repository is the **capture + store foundation**. It ships:

- a **`ValidatingWebhookConfiguration`** (wildcard-capable; default filter selects KRCI
  objects) that never blocks mutations (`failurePolicy: Ignore`, `timeoutSeconds: 1`);
- a **Vector** pipeline that reshapes the `AdmissionReview` payload into typed columns;
- a **PostgreSQL schema** (`audit_events`) — RANGE-partitioned by month, composite PK
  `(event_uid, received_at)`, `BEFORE INSERT` dedup, least-privilege append-only writer, and a
  default read view that hides dry-run previews;
- a **migration runner** (`krci-audit-migrate`) and Helm chart to deploy it all.

## Prerequisite

**cert-manager must already be installed in the cluster.** Kubernetes admission webhooks are
only ever called over TLS, so `kube-audit-rest`'s serving certificate and the webhook's
`caBundle` are issued and kept in sync by cert-manager (see `deploy-templates/templates/
certificate.yaml`). Without it, `helm install` will apply CRs cert-manager needs to reconcile
and the webhook will never become reachable.

## Components

- **[kube-audit-rest](https://github.com/RichardoC/kube-audit-rest)** — the
  `ValidatingWebhookConfiguration` target; logs the raw `AdmissionReview` and always allows
  the request.
- **[Vector](https://vector.dev/)** — sidecar that tails the log and ships shaped rows to
  PostgreSQL.
- **PostgreSQL** — the append-only, partitioned `audit_events` store (BYO, or provisioned via
  [Crunchydata's postgres-operator](https://github.com/CrunchyData/postgres-operator) or a
  plain in-cluster Deployment — see `db.mode` below).
- **[cert-manager](https://cert-manager.io/)** — issues and rotates the webhook's TLS
  certificate (prerequisite, see above).
- **[golang-migrate](https://github.com/golang-migrate/migrate)** — applies the embedded SQL
  migrations via the `krci-audit-migrate` runner.

## Store contract

| Column | Source (`AdmissionReview`) | Notes |
|---|---|---|
| `event_uid` | `request.uid` | logical event id + dedup key |
| `received_at` | `object.metadata.creationTimestamp` (CREATE) ‖ `requestReceivedTimestamp` | partition key |
| `operation`, `api_group`, `api_version`, `resource`, `kind`, `sub_resource` | `request.*` | |
| `namespace`, `name` | `request.*` (name falls back to object/oldObject) | |
| `object_uid` | object/oldObject `metadata.uid` | correlation column |
| `username`, `user_groups`, `user_extra` | `request.userInfo.*` | human vs `system:serviceaccount:` |
| `dry_run` | `request.dryRun` | stored, hidden from the default trail |
| `object`, `old_object`, `raw` | `request.object`/`oldObject` / whole payload | configurable; not searchable in v1 |

## Develop

See [CLAUDE.md](./CLAUDE.md). `make test` runs unit tests, Docker-backed store integration
tests, and helm render tests; `make build` produces the migrator binary.

## Configuration

The stored event set is configuration (`capture.filter` / `capture.rules` in the chart) and
can be changed with a `helm upgrade` — no code change. The object-body capture level
(`capture.level`) toggles between `metadata` (default) and `full`. Retention is enforced by
dropping whole monthly partitions (scheduled by an external rotation job), never row-by-row deletes.

### Database (`db.mode`)

Choose how PostgreSQL is supplied at deploy time:

- **`external`** (default) — bring your own DB: set `db.host` and `db.owner.secretName`.
- **`pgo`** — provision a Crunchydata `PostgresCluster` (needs the postgres-operator add-on).
- **`simple`** — provision a single in-cluster Postgres `Deployment` (dev/small installs).

```bash
helm install krci-audit deploy-templates -n krci-audit --set db.mode=pgo
helm install krci-audit deploy-templates -n krci-audit --set db.mode=simple
helm install krci-audit deploy-templates -n krci-audit \
  --set db.mode=external --set db.host=my-pg --set db.owner.secretName=my-pg-creds
```
