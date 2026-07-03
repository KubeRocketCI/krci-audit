// Package api is the krci-audit read API: an OpenAPI-defined, chi-served, read-only HTTP
// surface over the audit event store. The contract lives in oapi.yaml (single source of
// truth); oapi_gen.go is generated from it (never hand-edited). Thin handlers implement the
// generated StrictServerInterface and delegate to capability services in
// internal/services/<capability>, which run plain parameterized SQL over the audit_events_real
// view as the least-privilege audit_reader role.
package api
