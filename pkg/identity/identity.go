// Package identity classifies the acting identity recorded on an audit event
// (request.userInfo.username). It is a pure, dependency-free helper so the
// human/automation distinction is defined in exactly one place for every consumer of the
// store.
package identity

import "strings"

const (
	// serviceAccountPrefix is the exact Kubernetes ServiceAccount username prefix
	// (system:serviceaccount:<namespace>:<name>).
	serviceAccountPrefix = "system:serviceaccount:"

	// systemPrefix matches any Kubernetes system principal — ServiceAccounts AND other
	// system identities (system:kube-controller-manager, system:node:..., etc.). It is
	// deliberately broader than serviceAccountPrefix: every service account is a system
	// principal, but not every system principal is a service account.
	systemPrefix = "system:"
)

// Unknown is the sentinel actor used when a mutation carries no resolvable identity.
// Attribution is never dropped silently: the event is recorded with this actor and flagged
// instead.
const Unknown = "system:unknown"

// Class is the coarse classification of an actor.
type Class int

const (
	// ClassUnknown means the identity was empty/unresolved on the admission request.
	ClassUnknown Class = iota
	// ClassAutomation means a system principal acted (a ServiceAccount or another
	// system: identity such as a controller).
	ClassAutomation
	// ClassHuman means a human (typically an OIDC) user acted.
	ClassHuman
)

func (c Class) String() string {
	switch c {
	case ClassAutomation:
		return "automation"
	case ClassHuman:
		return "human"
	default:
		return "unknown"
	}
}

// IsServiceAccount reports whether username is specifically a Kubernetes ServiceAccount
// identity. It is narrower than IsAutomated: system:kube-controller-manager is automated
// but is not a ServiceAccount.
func IsServiceAccount(username string) bool {
	return strings.HasPrefix(username, serviceAccountPrefix)
}

// Classify maps a username to its Class.
func Classify(username string) Class {
	switch {
	case strings.TrimSpace(username) == "":
		return ClassUnknown
	case strings.HasPrefix(username, systemPrefix):
		return ClassAutomation
	default:
		return ClassHuman
	}
}

// IsAutomated reports whether the actor is a system principal (broader than ServiceAccounts).
func IsAutomated(username string) bool {
	return Classify(username) == ClassAutomation
}

// Normalize returns the username to record for an event. An empty/blank username is
// normalized to the Unknown sentinel so attribution is preserved rather than dropped.
func Normalize(username string) string {
	if strings.TrimSpace(username) == "" {
		return Unknown
	}
	return username
}

// Flagged reports whether an actor should be flagged for review because it could not be
// resolved to a concrete principal (empty or the Unknown sentinel).
func Flagged(username string) bool {
	u := strings.TrimSpace(username)
	return u == "" || u == Unknown
}
