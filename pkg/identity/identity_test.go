package identity

import "testing"

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
			if got := Classify(c.username); got != c.want {
				t.Fatalf("Classify(%q) = %v, want %v", c.username, got, c.want)
			}
		})
	}
}

// TestServiceAccountVsAutomation pins the intentional relationship between the two rules:
// every ServiceAccount is automation, but automation is broader (any system: principal).
func TestServiceAccountVsAutomation(t *testing.T) {
	sa := "system:serviceaccount:edp:operator"
	if !IsServiceAccount(sa) || !IsAutomated(sa) {
		t.Fatal("a ServiceAccount must be both a ServiceAccount and automation")
	}

	sysNonSA := "system:kube-controller-manager"
	if IsServiceAccount(sysNonSA) {
		t.Fatal("a non-SA system principal must NOT be a ServiceAccount")
	}
	if !IsAutomated(sysNonSA) {
		t.Fatal("a non-SA system principal must still be automation")
	}

	human := "dev@example.com"
	if IsServiceAccount(human) || IsAutomated(human) {
		t.Fatal("a human must be neither a ServiceAccount nor automation")
	}
}

// TestNormalize covers DT-5 / DT-24: an unresolved actor is never dropped — it is
// preserved as the Unknown sentinel.
func TestNormalize(t *testing.T) {
	if got := Normalize(""); got != Unknown {
		t.Fatalf("Normalize(\"\") = %q, want %q", got, Unknown)
	}
	if got := Normalize("   "); got != Unknown {
		t.Fatalf("Normalize(blank) = %q, want %q", got, Unknown)
	}
	if got := Normalize("dev@example.com"); got != "dev@example.com" {
		t.Fatalf("Normalize preserved value = %q", got)
	}
}

func TestFlagged(t *testing.T) {
	if !Flagged("") || !Flagged(Unknown) {
		t.Fatal("empty and Unknown must be flagged")
	}
	if Flagged("system:serviceaccount:edp:operator") || Flagged("dev@example.com") {
		t.Fatal("resolved identities must not be flagged")
	}
}
