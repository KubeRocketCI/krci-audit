package api

import "testing"

func TestClampPagination(t *testing.T) {
	p := func(v int) *int { return &v }

	tests := []struct {
		name          string
		page, perPage *int
		wantP, wantPP int
	}{
		{"defaults when nil", nil, nil, 1, 20},
		{"custom values", p(3), p(50), 3, 50},
		{"perPage capped at 100", p(1), p(500), 1, 100},
		{"page floored at 1", p(0), p(20), 1, 20},
		{"negative page floored", p(-5), p(20), 1, 20},
		{"perPage < 1 falls back to default", p(1), p(0), 1, 20},
		{"page capped at maxPage", p(1000000), p(20), maxPage, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotP, gotPP := clampPagination(tt.page, tt.perPage)
			if gotP != tt.wantP || gotPP != tt.wantPP {
				t.Fatalf("clampPagination = (%d,%d), want (%d,%d)", gotP, gotPP, tt.wantP, tt.wantPP)
			}
		})
	}
}
