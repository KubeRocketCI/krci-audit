package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Uses assert (not require, the package convention elsewhere) deliberately: this file mirrors
// the shared cross-repo test-format reference verbatim.
func TestClampPagination(t *testing.T) {
	p := ptrTo[int]

	tests := []struct {
		name        string
		page        *int
		perPage     *int
		wantPage    int
		wantPerPage int
	}{
		{name: "defaults when nil", page: nil, perPage: nil, wantPage: 1, wantPerPage: 20},
		{name: "custom values", page: p(3), perPage: p(50), wantPage: 3, wantPerPage: 50},
		{name: "perPage capped at 100", page: p(1), perPage: p(500), wantPage: 1, wantPerPage: 100},
		{name: "page floored at 1", page: p(0), perPage: p(20), wantPage: 1, wantPerPage: 20},
		{name: "negative page floored", page: p(-5), perPage: p(20), wantPage: 1, wantPerPage: 20},
		{name: "perPage < 1 falls back to default", page: p(1), perPage: p(0), wantPage: 1, wantPerPage: 20},
		{name: "page capped at maxPage", page: p(1000000), perPage: p(20), wantPage: maxPage, wantPerPage: 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPerPage := clampPagination(tt.page, tt.perPage)

			assert.Equal(t, tt.wantPage, gotPage)
			assert.Equal(t, tt.wantPerPage, gotPerPage)
		})
	}
}
