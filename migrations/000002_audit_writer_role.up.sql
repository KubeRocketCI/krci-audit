-- Least-privilege writer role for the ingestion path (Vector postgres sink).
-- Append-only integrity: the writer can only INSERT and SELECT; no interactive role
-- (including this one) can UPDATE or DELETE individual events.
--
-- The role is created NOLOGIN here; the deployment attaches LOGIN + a password from a
-- managed Secret via `ALTER ROLE audit_writer LOGIN PASSWORD ...`. Keeping credentials out
-- of the schema migration avoids baking secrets into version control.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_writer') THEN
        CREATE ROLE audit_writer NOLOGIN;
    END IF;
END
$$;

-- Allow the writer to connect (explicit, in case PUBLIC's default CONNECT was revoked).
DO $$ BEGIN EXECUTE format('GRANT CONNECT ON DATABASE %I TO audit_writer', current_database()); END $$;

GRANT USAGE ON SCHEMA public TO audit_writer;

-- INSERT/SELECT on the partitioned parent covers inserts routed through it and reads.
GRANT INSERT, SELECT ON audit_events TO audit_writer;
GRANT SELECT ON audit_events_real TO audit_writer;

-- Future partitions (created ahead by the rotation job) inherit INSERT/SELECT so the
-- writer keeps working across rotation without a re-grant, and stays non-mutating.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT INSERT, SELECT ON TABLES TO audit_writer;
