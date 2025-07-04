package scopes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFeatureEnabled verifies the expected behavior of the scope feature flag.
func TestFeatureEnabled(t *testing.T) {
	require.Error(t, AssertFeatureEnabled())
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	require.NoError(t, AssertFeatureEnabled())

}
