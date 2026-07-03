package migrate

// Docker-free unit test for the pure branching in RunCLI. Applying migrations (Up/Down) and
// setting role passwords need a real Postgres and are covered by the store integration suite
// (`make test-integration`); the direction-dispatch/validation logic itself does not.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunCLIRejectsUnknownDirection(t *testing.T) {
	err := RunCLI(context.Background(), "sideways", "pgx5://u:p@h:5432/d", "", "")
	require.Error(t, err, "expected an error for an unrecognized direction")
}
