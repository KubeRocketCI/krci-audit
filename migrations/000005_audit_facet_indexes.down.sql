-- Reverse 000005: drop the facet indexes.
DROP INDEX IF EXISTS audit_events_namespace;
DROP INDEX IF EXISTS audit_events_kind;
