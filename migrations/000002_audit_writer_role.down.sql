-- Reverse 000002: revoke grants and drop the writer role.
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE INSERT, SELECT ON TABLES FROM audit_writer;
REVOKE SELECT ON audit_events_real FROM audit_writer;
REVOKE INSERT, SELECT ON audit_events FROM audit_writer;
REVOKE USAGE ON SCHEMA public FROM audit_writer;
DO $$ BEGIN EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM audit_writer', current_database()); END $$;
DROP ROLE IF EXISTS audit_writer;
