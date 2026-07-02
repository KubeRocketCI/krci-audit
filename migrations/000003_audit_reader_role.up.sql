-- Least-privilege read-only role for a read/export API. Strictly separate from audit_writer:
-- the read path can SELECT but never INSERT/UPDATE/DELETE, so serving queries can never
-- mutate the trail.
--
-- Created NOLOGIN here; the deployment attaches LOGIN + a password from a managed Secret,
-- exactly as it does for audit_writer — keeping credentials out of the schema migration.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_reader') THEN
        CREATE ROLE audit_reader NOLOGIN;
    END IF;
END
$$;

DO $$ BEGIN EXECUTE format('GRANT CONNECT ON DATABASE %I TO audit_reader', current_database()); END $$;

GRANT USAGE ON SCHEMA public TO audit_reader;

-- SELECT on the partitioned parent (covers all partitions) and the default read view.
GRANT SELECT ON audit_events TO audit_reader;
GRANT SELECT ON audit_events_real TO audit_reader;

-- Future partitions (created ahead by the rotation job) inherit SELECT so the reader
-- keeps working across rotation, and stays non-mutating.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO audit_reader;
