-- Facets API: distinct-value lookups (namespace, kind, actor) drive a loose index scan
-- (recursive CTE walking the index), so the cost is O(distinct values * log N) rather than a
-- full table scan. The username facet is already served by the existing
-- audit_events_username_ts index (username leads the composite key); namespace and kind have
-- no leading index yet, so add one each here. Partial WHERE dry_run = false matches the
-- facets query predicate exactly, so the planner can use these indexes for an index-only
-- loose scan. Applies to the partitioned parent (cascades to all partitions).
CREATE INDEX IF NOT EXISTS audit_events_namespace ON audit_events (namespace) WHERE dry_run = false;
CREATE INDEX IF NOT EXISTS audit_events_kind      ON audit_events (kind)      WHERE dry_run = false;
