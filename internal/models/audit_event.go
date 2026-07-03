// Package models holds the krci-audit domain types — the single Go home for the audit
// event shape and its vocabulary. Every consumer imports these types instead of
// re-declaring column names, operations, or capture levels, so the contract with the
// audit_events table lives in exactly one place.
//
// The db struct tags on AuditEvent are the source of truth for the column set; store and
// tests derive their column lists from AllColumns()/LiftedColumns() rather than hand-typing
// parallel lists that can silently drift from the migration.
package models

import (
	"encoding/json"
	"reflect"
	"time"
)

// Operation is the admission operation recorded on an event (request.operation).
type Operation string

// Supported Operation values.
const (
	OperationCreate  Operation = "CREATE"
	OperationUpdate  Operation = "UPDATE"
	OperationDelete  Operation = "DELETE"
	OperationConnect Operation = "CONNECT"
)

// AllOperations returns every admission operation value krci-audit records. This is the
// source of truth other packages (e.g. API filter validation) should derive from rather than
// hand-listing the values again.
func AllOperations() []Operation {
	return []Operation{OperationCreate, OperationUpdate, OperationDelete, OperationConnect}
}

// CaptureLevel controls how much of the object/oldObject body is stored. Metadata-only is
// the default (bounds size and PII exposure); full stores the whole body.
type CaptureLevel string

// Supported CaptureLevel values.
const (
	CaptureLevelMetadata CaptureLevel = "metadata"
	CaptureLevelFull     CaptureLevel = "full"
)

// AuditEvent mirrors one row of the audit_events table. Field order and db tags match
// migrations/000001_audit_events.up.sql exactly. Body columns (object/old_object/raw) are
// stored and retrievable but are NOT part of the searchable "lifted" query surface in v1.
type AuditEvent struct {
	EventUID    string          `db:"event_uid"`
	ReceivedAt  time.Time       `db:"received_at"`
	Operation   Operation       `db:"operation"`
	APIGroup    string          `db:"api_group"`
	APIVersion  string          `db:"api_version"`
	Resource    string          `db:"resource"`
	Kind        string          `db:"kind"`
	SubResource *string         `db:"sub_resource"`
	Namespace   string          `db:"namespace"`
	Name        string          `db:"name"`
	ObjectUID   *string         `db:"object_uid"`
	Username    string          `db:"username"`
	UserGroups  json.RawMessage `db:"user_groups"`
	UserExtra   json.RawMessage `db:"user_extra"`
	DryRun      bool            `db:"dry_run"`
	Object      json.RawMessage `db:"object" body:"true"`
	OldObject   json.RawMessage `db:"old_object" body:"true"`
	Raw         json.RawMessage `db:"raw" body:"true"`
}

// bodyColumns is derived once from the struct tags; a column is a "body" (non-searchable)
// column when its field carries `body:"true"`.
var allColumns, liftedColumns = deriveColumns()

func deriveColumns() (all, lifted []string) {
	t := reflect.TypeOf(AuditEvent{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		col := f.Tag.Get("db")
		if col == "" {
			continue
		}
		all = append(all, col)
		if f.Tag.Get("body") != "true" {
			lifted = append(lifted, col)
		}
	}
	return all, lifted
}

// AllColumns returns every audit_events column name, in table order. This is the source of
// truth the store schema and tests validate against the live database.
func AllColumns() []string { return append([]string(nil), allColumns...) }

// LiftedColumns returns the searchable/typed query surface (v1): every column except the
// stored-but-not-searched bodies (object, old_object, raw).
func LiftedColumns() []string { return append([]string(nil), liftedColumns...) }
