package structured

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaFingerprint(t *testing.T) {
	t.Parallel()

	fp := SchemaFingerprint([]byte(`{"type":"object"}`))

	require.NotEmpty(t, fp)
	require.Equal(t, fp, SchemaFingerprint([]byte(`{"type":"object"}`)), "stable for identical input")
	require.NotEqual(t, fp, SchemaFingerprint([]byte(`{"type":"array"}`)), "differs for different input")
}
