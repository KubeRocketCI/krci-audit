package identity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClassify covers DT-2 (automated identity distinguishable) and DT-5 (unresolved
// identity), plus the human case (DT-1) and the system-but-not-ServiceAccount case.
func TestClassify(t *testing.T) {
	cases := []struct {
		name     string
		username string
		want     Class
	}{
		{"human OIDC", "dev@example.com", ClassHuman},
		{"service account", "system:serviceaccount:edp:operator", ClassAutomation},
		{"other system principal", "system:kube-controller-manager", ClassAutomation},
		{"empty", "", ClassUnknown},
		{"blank", "   ", ClassUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, Classify(c.username))
		})
	}
}

// TestServiceAccountVsAutomation pins the intentional relationship between the two rules:
// every ServiceAccount is automation, but automation is broader (any system: principal).
func TestServiceAccountVsAutomation(t *testing.T) {
	sa := "system:serviceaccount:edp:operator"
	require.True(t, IsServiceAccount(sa), "a ServiceAccount must be a ServiceAccount")
	require.True(t, IsAutomated(sa), "a ServiceAccount must be automation")

	sysNonSA := "system:kube-controller-manager"
	require.False(t, IsServiceAccount(sysNonSA), "a non-SA system principal must NOT be a ServiceAccount")
	require.True(t, IsAutomated(sysNonSA), "a non-SA system principal must still be automation")

	human := "dev@example.com"
	require.False(t, IsServiceAccount(human), "a human must not be a ServiceAccount")
	require.False(t, IsAutomated(human), "a human must not be automation")
}

// TestNormalize covers DT-5 / DT-24: an unresolved actor is never dropped — it is
// preserved as the Unknown sentinel.
func TestNormalize(t *testing.T) {
	require.Equal(t, Unknown, Normalize(""))
	require.Equal(t, Unknown, Normalize("   "))
	require.Equal(t, "dev@example.com", Normalize("dev@example.com"))
}

func TestClassString(t *testing.T) {
	cases := []struct {
		class Class
		want  string
	}{
		{ClassAutomation, "automation"},
		{ClassHuman, "human"},
		{ClassUnknown, "unknown"},
		{Class(99), "unknown"}, // any unrecognized value falls back to "unknown"
	}
	for _, c := range cases {
		require.Equal(t, c.want, c.class.String())
	}
}

func TestFlagged(t *testing.T) {
	require.True(t, Flagged(""), "empty must be flagged")
	require.True(t, Flagged(Unknown), "Unknown must be flagged")
	require.False(t, Flagged("system:serviceaccount:edp:operator"), "resolved identities must not be flagged")
	require.False(t, Flagged("dev@example.com"), "resolved identities must not be flagged")
}
