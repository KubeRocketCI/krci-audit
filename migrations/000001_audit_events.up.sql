-- krci-audit: platform-agnostic Kubernetes admission audit store.
--
-- One row per admission event, aligned to the Kubernetes AdmissionReview payload
-- (admission.k8s.io/v1) as captured verbatim by kube-audit-rest. Column names mirror
-- payload paths so the Vector `postgres` sink can map them directly with
-- jsonb_populate_record (NO surrogate id column — every column comes from the event).
--
-- Identity model:
--   * event_uid  = request.uid                    — the ADMISSION EVENT identity + dedup key
--   * object_uid = object/oldObject.metadata.uid  — the OBJECT-INSTANCE correlation column
-- A generic "who changed what" log stores many events per object (CREATE, N*UPDATE, DELETE),
-- so the row identity is the event, not the object.
--
-- Storage: RANGE-partitioned by received_at (monthly). PostgreSQL requires the partition
-- key in the primary key, hence the composite PK (event_uid, received_at). received_at is
-- deterministic per event (supplied by the ingestion path), so dedup is unaffected.

-- ---------------------------------------------------------------------------
-- 1. Partitioned audit_events table.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_events (
    event_uid    TEXT        NOT NULL,               -- request.uid (logical id + dedup key)
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now(), -- creationTimestamp(CREATE) | requestReceivedTimestamp; partition key
    operation    TEXT        NOT NULL,               -- CREATE | UPDATE | DELETE | CONNECT
    api_group    TEXT        NOT NULL DEFAULT '',     -- request.resource.group ('' for core)
    api_version  TEXT        NOT NULL,               -- request.resource.version
    resource     TEXT        NOT NULL,               -- request.resource.resource (plural, e.g. codebases)
    kind         TEXT        NOT NULL,               -- request.kind.kind
    sub_resource TEXT,                                -- request.subResource; null unless a subresource rule is enabled
    namespace    TEXT        NOT NULL DEFAULT '',     -- request.namespace ('' for cluster-scoped)
    name         TEXT        NOT NULL,               -- request.name | object/oldObject metadata.name
    object_uid   TEXT,                                -- object/oldObject metadata.uid (correlation)
    username     TEXT        NOT NULL,               -- request.userInfo.username (WHO)
    user_groups  JSONB,                               -- request.userInfo.groups
    user_extra   JSONB,                               -- request.userInfo.extra
    dry_run      BOOLEAN     NOT NULL DEFAULT false,  -- request.dryRun
    object       JSONB,                               -- request.object; nullable; configurable full vs metadata-only
    old_object   JSONB,                               -- request.oldObject; nullable (null on CREATE)
    raw          JSONB,                               -- optional full-fidelity payload; off by default
    PRIMARY KEY (event_uid, received_at)
) PARTITION BY RANGE (received_at);

-- ---------------------------------------------------------------------------
-- 2. Query surface = lifted typed columns only (v1). Partial indexes exclude
--    dry-run so the default hot path never scans previews. Trailing received_at
--    DESC serves newest-first and intra-partition time ranges; partition
--    boundaries provide the coarse time index (no BRIN needed).
--    NOTE: object/old_object/raw are stored + retrievable but NOT searchable in
--    v1 (no JSON/GIN path index) — a deferred enhancement.
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS audit_events_gvk_name_ts
    ON audit_events (api_group, resource, namespace, name, received_at DESC) WHERE dry_run = false;
CREATE INDEX IF NOT EXISTS audit_events_username_ts
    ON audit_events (username, received_at DESC) WHERE dry_run = false;
CREATE INDEX IF NOT EXISTS audit_events_object_uid_ts
    ON audit_events (object_uid, received_at DESC) WHERE dry_run = false;
-- Compliance "who previewed destructive changes" — the only fast path over dry-run rows.
CREATE INDEX IF NOT EXISTS audit_events_username_ts_dry
    ON audit_events (username, received_at DESC) WHERE dry_run = true;

-- ---------------------------------------------------------------------------
-- 3. Default read surface for all consumers. Dry-run events are stored but must
--    never appear in the normal trail; consumers query this view, and only an
--    explicit dry_run = true predicate reveals previews.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW audit_events_real AS
    SELECT * FROM audit_events WHERE dry_run = false;

-- ---------------------------------------------------------------------------
-- 4. Dedup trigger. The Vector postgres sink has no ON CONFLICT and a single
--    unique violation fails the whole insert batch; the API server may also retry
--    a webhook. This BEFORE INSERT row trigger (supported on partitioned parents
--    since PG13; cascades to all partitions) silently swallows a duplicate
--    (event_uid, received_at) by returning NULL — no error surfaced to ingestion.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION audit_events_dedup() RETURNS trigger AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM audit_events
        WHERE event_uid = NEW.event_uid AND received_at = NEW.received_at
    ) THEN
        RETURN NULL;  -- duplicate admission delivery (apiserver/webhook retry): skip, no error
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER audit_events_dedup_trg
    BEFORE INSERT ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_dedup();

-- ---------------------------------------------------------------------------
-- 5. Partition management primitive. Retention is enforced by dropping/detaching
--    whole partitions (a constant-time metadata op), NOT row-by-row DELETE.
--    Create-ahead / drop-expired scheduling is performed by an external rotation job;
--    this migration provides the primitive and the initial partitions.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION audit_ensure_partition(p_ts TIMESTAMPTZ)
RETURNS TEXT AS $$
DECLARE
    v_start DATE := date_trunc('month', p_ts)::date;
    v_end   DATE := (date_trunc('month', p_ts) + INTERVAL '1 month')::date;
    v_name  TEXT := format('audit_events_%s', to_char(v_start, 'YYYYMM'));
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = v_name) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF audit_events FOR VALUES FROM (%L) TO (%L)',
            v_name, v_start, v_end
        );
    END IF;
    RETURN v_name;
END;
$$ LANGUAGE plpgsql;

-- Initial runway: current month + two ahead, so ingestion has partitions until the
-- rotation job takes over create-ahead.
SELECT audit_ensure_partition(now());
SELECT audit_ensure_partition(now() + INTERVAL '1 month');
SELECT audit_ensure_partition(now() + INTERVAL '2 months');
