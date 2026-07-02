-- Reverse 000001: drop the read view, the audit_events table (CASCADE removes all
-- partitions and partitioned indexes), and the helper/dedup functions.
DROP VIEW IF EXISTS audit_events_real;
DROP TABLE IF EXISTS audit_events CASCADE;
DROP FUNCTION IF EXISTS audit_ensure_partition(TIMESTAMPTZ);
DROP FUNCTION IF EXISTS audit_events_dedup();
