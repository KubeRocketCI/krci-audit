package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestColumnsDerivedFromStruct(t *testing.T) {
	all := AllColumns()
	// Spot-check ordering matches the DDL (event_uid first, received_at second — the PK).
	require.Equal(t, []string{"event_uid", "received_at"}, all[:2])
	require.Len(t, all, 18)
}

func TestLiftedExcludesBodies(t *testing.T) {
	lifted := LiftedColumns()
	for _, body := range []string{"object", "old_object", "raw"} {
		require.NotContains(t, lifted, body, "lifted query surface must not include body column %q", body)
	}

	// Lifted must be a strict subset of all columns.
	all := AllColumns()
	for _, c := range lifted {
		require.Contains(t, all, c, "lifted column %q is not a real column", c)
	}
	require.Len(t, lifted, len(all)-3, "expected lifted = all - 3 bodies")
}

func TestAllOperations(t *testing.T) {
	want := []Operation{OperationCreate, OperationUpdate, OperationDelete, OperationConnect}
	require.Equal(t, want, AllOperations())
}

func TestReturnedSlicesAreCopies(t *testing.T) {
	a := AllColumns()
	a[0] = "mutated"
	require.Equal(t, "event_uid", AllColumns()[0], "AllColumns must return a defensive copy")
}
