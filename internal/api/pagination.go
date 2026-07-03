package api

// Pagination defaults and bounds. Default page is 1, default perPage is 20, max perPage is 100.
// maxPage bounds the OFFSET computed from it, so a request can't force an unbounded scan over
// audit_events, which is append-only and expected to grow indefinitely.
//
// TODO: OFFSET still costs a full scan-and-discard up to maxPage*maxPerPage rows; move to
// keyset pagination (received_at < cursor) once that cost becomes real at production volume.
const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
	maxPage        = 10000
)

// clampPagination applies default values and bounds to pagination parameters.
func clampPagination(page, perPage *int) (int, int) {
	p := defaultPage
	if page != nil {
		p = *page
	}
	if p < 1 {
		p = defaultPage
	}
	if p > maxPage {
		p = maxPage
	}

	pp := defaultPerPage
	if perPage != nil {
		pp = *perPage
	}
	if pp < 1 {
		pp = defaultPerPage
	}
	if pp > maxPerPage {
		pp = maxPerPage
	}

	return p, pp
}
