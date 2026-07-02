package models

import (
	"slices"
	"testing"
)

func TestColumnsDerivedFromStruct(t *testing.T) {
	all := AllColumns()
	// Spot-check ordering matches the DDL (event_uid first, received_at second — the PK).
	if all[0] != "event_uid" || all[1] != "received_at" {
		t.Fatalf("unexpected leading columns: %v", all[:2])
	}
	if len(all) != 18 {
		t.Fatalf("expected 18 columns, got %d: %v", len(all), all)
	}
}

func TestLiftedExcludesBodies(t *testing.T) {
	lifted := LiftedColumns()
	for _, body := range []string{"object", "old_object", "raw"} {
		if slices.Contains(lifted, body) {
			t.Fatalf("lifted query surface must not include body column %q", body)
		}
	}
	// Lifted must be a strict subset of all columns.
	all := AllColumns()
	for _, c := range lifted {
		if !slices.Contains(all, c) {
			t.Fatalf("lifted column %q is not a real column", c)
		}
	}
	if len(lifted) != len(all)-3 {
		t.Fatalf("expected lifted = all - 3 bodies; all=%d lifted=%d", len(all), len(lifted))
	}
}

func TestReturnedSlicesAreCopies(t *testing.T) {
	a := AllColumns()
	a[0] = "mutated"
	if AllColumns()[0] != "event_uid" {
		t.Fatal("AllColumns must return a defensive copy")
	}
}
