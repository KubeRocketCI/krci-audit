-- Scheduled partition rotation, invoked by the retention CronJob (which connects as the schema
-- owner). Encapsulating the policy here keeps the CronJob a thin `SELECT` caller and keeps the
-- rotation rules in versioned, owner-run SQL alongside the schema they operate on.
--
-- Two operations per pass:
--   create-ahead : ensure the current month + N future monthly partitions exist (ingestion
--                  runway), reusing audit_ensure_partition (migration 000001).
--   drop-expired : DROP whole partitions whose entire month is older than the retention window.
-- Whole-partition metadata operations only — never row-level DELETE — so append-only holds.

CREATE OR REPLACE FUNCTION audit_rotate_partitions(
    p_retention_months    INTEGER,
    p_create_ahead_months INTEGER DEFAULT 3
)
RETURNS TABLE(action TEXT, partition TEXT) AS $$
DECLARE
    v_month  DATE := date_trunc('month', now())::date;
    -- A partition's start month is expired once start + 1 month <= v_month - retention, i.e.
    -- start <= v_month - (retention + 1) months: every record it can hold is already older than
    -- the window. Folding the +1 month into the cutoff keeps the boundary month retained until
    -- it fully elapses (records inside the window are never dropped) and lets the drop query use
    -- a plain start-month comparison.
    v_cutoff DATE := v_month - make_interval(months => p_retention_months + 1);
    v_i      INTEGER;
    v_rec    RECORD;
BEGIN
    IF p_retention_months < 1 THEN
        RAISE EXCEPTION 'p_retention_months must be >= 1, got %', p_retention_months;
    END IF;
    IF p_create_ahead_months < 0 THEN
        RAISE EXCEPTION 'p_create_ahead_months must be >= 0, got %', p_create_ahead_months;
    END IF;

    FOR v_i IN 0..p_create_ahead_months LOOP
        action    := 'ensured';
        partition := audit_ensure_partition(v_month + make_interval(months => v_i));
        RETURN NEXT;
    END LOOP;

    FOR v_rec IN
        SELECT c.relname AS name
        FROM pg_inherits i
        JOIN pg_class p ON p.oid = i.inhparent
        JOIN pg_class c ON c.oid = i.inhrelid
        WHERE p.relname = 'audit_events'
          AND c.relname ~ '^audit_events_\d{6}$'
          AND to_date(right(c.relname, 6), 'YYYYMM') <= v_cutoff
        ORDER BY c.relname
    LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I', v_rec.name);
        action    := 'dropped';
        partition := v_rec.name;
        RETURN NEXT;
    END LOOP;
END;
$$ LANGUAGE plpgsql;
