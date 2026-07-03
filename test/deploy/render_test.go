package deploy_test

// Config-layer tests for the krci-audit Helm chart. They render the chart with `helm
// template` and assert the capture/ship guarantees that live in configuration rather than
// Go code: the non-blocking webhook (failurePolicy Ignore, timeoutSeconds 1), the
// KRCI-default store filter, the one-row-per-statement postgres sink, the spec-level
// subresource default, and the metadata-vs-full capture-level toggle.
//
// Requires the `helm` binary; if it is absent the test is skipped (not failed).

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func chartDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// test/deploy -> repo root -> deploy-templates
	return filepath.Join(filepath.Dir(file), "..", "..", "deploy-templates")
}

// render renders with db.mode=simple, which is self-contained (no required external
// inputs); the capture/ship assertions below are DB-mode-independent.
func render(t *testing.T, extraArgs ...string) string {
	t.Helper()
	return renderMode(t, "simple", extraArgs...)
}

// renderMode renders a specific db.mode with any inputs it requires.
func renderMode(t *testing.T, mode string, extraArgs ...string) string {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed, skipping chart render tests")
	}
	args := []string{"template", "krci-audit", chartDir(t), "--namespace", "krci-audit", "--set", "db.mode=" + mode}
	out, err := exec.Command("helm", append(args, extraArgs...)...).CombinedOutput()
	require.NoErrorf(t, err, "helm template (mode=%s) failed: %s", mode, out)
	return string(out)
}

// TestDBModes: external provisions no DB; pgo emits a PostgresCluster; simple emits an
// in-cluster Postgres Deployment. All three wire Vector to the audit_writer role.
func TestDBModes(t *testing.T) {
	external := renderMode(t, "external", "--set", "db.host=my-pg", "--set", "db.owner.secretName=my-pg-creds")
	require.NotContains(t, external, "kind: PostgresCluster", "external provisions no DB")
	require.NotContains(t, external, "name: krci-audit-db", "external provisions no DB Deployment")
	require.Contains(t, external, `value: "my-pg"`, "external wires the provided host")

	pgo := renderMode(t, "pgo")
	require.Contains(t, pgo, "kind: PostgresCluster")
	require.Contains(t, pgo, "krci-audit-pguser-krci-audit", "owner creds from the pgo pguser Secret")

	simple := renderMode(t, "simple")
	require.Contains(t, simple, "image: \"postgres:16-alpine\"", "simple provisions a plain Postgres")
	require.Contains(t, simple, "krci-audit-writer", "writer Secret is chart-managed")

	// The writer always connects as the least-privilege audit_writer, never the owner.
	for _, out := range []string{external, pgo, simple} {
		require.Contains(t, out, "value: \"audit_writer\"")
	}
}

func TestWebhookNeverBlocks(t *testing.T) {
	out := render(t)
	require.Contains(t, out, "failurePolicy: Ignore", "webhook must not block platform mutations")
	require.Contains(t, out, "timeoutSeconds: 1", "webhook must use a short timeout")
	require.Contains(t, out, "cert-manager.io/inject-ca-from", "caBundle must be cert-manager-managed")
}

func TestDefaultFilterIsKRCI(t *testing.T) {
	out := render(t)
	require.Contains(t, out, `.request.resource.group == "v2.edp.epam.com"`, "KRCI CRDs stored by default")
	require.Contains(t, out, `.request.resource.resource == "pipelineruns"`, "PipelineRun stored by default")
}

func TestPostgresSinkOneRowPerStatement(t *testing.T) {
	out := render(t)
	require.Contains(t, out, "max_events: 1", "dedup no-op must not fail a whole batch")
	require.Contains(t, out, "table: audit_events")
}

func TestSubresourcePolicySpecLevelByDefault(t *testing.T) {
	out := render(t)
	require.Contains(t, out, `if .request.subResource != null && .request.subResource != ""`)
	require.Contains(t, out, "if !sub_ok { stored = false }")

	withStatus := render(t, "--set", "capture.includeSubresources={status}")
	require.Contains(t, withStatus, `if .request.subResource == "status" { sub_ok = true }`)
}

// The read API renders as a SEPARATE Deployment + ClusterIP Service (no Ingress), connects as
// the least-privilege audit_reader role, and the migration Job provisions the reader LOGIN
// password. Its selector is disjoint from the capture pod's so neither Service cross-routes.
func TestReadAPISurface(t *testing.T) {
	out := render(t)

	require.Contains(t, out, "name: krci-audit-api", "API Deployment/Service are named distinctly from the capture pod")
	require.Contains(t, out, "app.kubernetes.io/component: read-api")
	require.Contains(t, out, "containerPort: 8080", "API serves on 8080")
	require.Contains(t, out, `value: "audit_reader"`, "API connects as the SELECT-only reader role")
	require.Contains(t, out, "AUDIT_READER_PASSWORD", "migration Job provisions the reader login")
	require.Contains(t, out, "name: krci-audit-reader", "reader password Secret is chart-managed")
	require.NotContains(t, out, "kind: Ingress", "API is reachable intra-cluster only, no Ingress")
}

// Disabling the API removes all of its objects and the reader-password provisioning.
func TestReadAPIDisabled(t *testing.T) {
	out := render(t, "--set", "api.enabled=false")
	require.NotContains(t, out, "name: krci-audit-api")
	require.NotContains(t, out, "AUDIT_READER_PASSWORD")
	require.NotContains(t, out, "name: krci-audit-reader")
}

func TestCaptureLevelToggle(t *testing.T) {
	meta := render(t)
	require.Contains(t, meta, `obj = { "apiVersion": .request.object.apiVersion`, "metadata-only trims the body")

	full := render(t, "--set", "capture.level=full")
	require.Contains(t, full, "obj = .request.object", "full capture stores the whole object")
}
