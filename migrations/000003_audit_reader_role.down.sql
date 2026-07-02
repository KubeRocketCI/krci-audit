-- Reverse 000003: revoke grants and drop the reader role.
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT ON TABLES FROM audit_reader;
REVOKE SELECT ON audit_events_real FROM audit_reader;
REVOKE SELECT ON audit_events FROM audit_reader;
REVOKE USAGE ON SCHEMA public FROM audit_reader;
DO $$ BEGIN EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM audit_reader', current_database()); END $$;
DROP ROLE IF EXISTS audit_reader;
