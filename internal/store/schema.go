// Package store describes the persistence contract: the database object names the
// migrations create and that consumers (e.g. a read API) depend on. Keeping these as
// exported constants gives downstream code a single, typo-proof reference to the schema.
//
// The column vocabulary itself lives in internal/models (AuditEvent + AllColumns/
// LiftedColumns), derived from the struct tags, so there is one authoritative source rather
// than a hand-maintained list here.
package store

const (
	// EventsTable is the RANGE-partitioned parent table holding one row per admission event.
	EventsTable = "audit_events"

	// RealView is the default read surface for all consumers; it excludes dry-run events.
	// Consumers SHOULD query this view (or apply WHERE dry_run = false) so previews are
	// never mistaken for real actions.
	RealView = "audit_events_real"

	// WriterRole is the least-privilege ingestion role (INSERT/SELECT only) that enforces
	// append-only integrity at the database layer.
	WriterRole = "audit_writer"

	// ReaderRole is the least-privilege read-only role (SELECT only) a read/export API
	// connects as — strictly separate from WriterRole so the read path can never mutate.
	ReaderRole = "audit_reader"

	// EnsurePartitionFn creates the monthly partition covering a timestamp if absent.
	// It is the primitive a scheduled rotation job uses for create-ahead scheduling.
	EnsurePartitionFn = "audit_ensure_partition"
)
