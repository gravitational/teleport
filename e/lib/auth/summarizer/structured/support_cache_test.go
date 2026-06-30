package structured

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestSupportCache(t *testing.T) {
	t.Parallel()

	const schema = "schema-fingerprint"

	t.Run("remembers verdict until the TTL elapses", func(t *testing.T) {
		clock := clockwork.NewFakeClock()

		c := NewSupportCache(clock, time.Hour)
		const key = "arn:aws:bedrock:us-east-1:123456789012:application-inference-profile/a1b2c3d4e5f6"

		require.False(t, c.Unsupported(key, schema), "unknown before any verdict")
		c.MarkUnsupported(key, schema)
		require.True(t, c.Unsupported(key, schema), "remembered after marking")

		clock.Advance(59 * time.Minute)
		require.True(t, c.Unsupported(key, schema), "still within TTL")

		clock.Advance(2 * time.Minute)
		require.False(t, c.Unsupported(key, schema), "expired after TTL, re-probe")
	})

	t.Run("tracks verdicts per key", func(t *testing.T) {
		clock := clockwork.NewFakeClock()

		c := NewSupportCache(clock, time.Hour)
		c.MarkUnsupported("model-a", schema)

		require.True(t, c.Unsupported("model-a", schema))
		require.False(t, c.Unsupported("model-b", schema))
	})

	t.Run("tracks verdicts per schema", func(t *testing.T) {
		clock := clockwork.NewFakeClock()

		c := NewSupportCache(clock, time.Hour)
		c.MarkUnsupported("model-a", "schema-1")

		require.True(t, c.Unsupported("model-a", "schema-1"))
		require.False(t, c.Unsupported("model-a", "schema-2"), "a verdict for one schema must not disable native for another")
	})

	t.Run("nil cache is a no-op", func(t *testing.T) {
		var c *SupportCache

		require.False(t, c.Unsupported("x", schema))

		c.MarkUnsupported("x", schema) // must not panic
	})
}

func TestUseNative(t *testing.T) {
	t.Parallel()

	const key, schema = "model", "schema-fingerprint"

	c := NewSupportCache(clockwork.NewFakeClock(), time.Hour)

	require.True(t, c.UseNative(SupportYes, key, schema), "known-supported always uses native")
	require.False(t, c.UseNative(SupportNo, key, schema), "known-unsupported never uses native")
	require.True(t, c.UseNative(SupportUnknown, key, schema), "unknown probes when no verdict is cached")

	c.MarkUnsupported(key, schema)

	require.False(t, c.UseNative(SupportUnknown, key, schema), "unknown skips native once a verdict is cached")
	require.True(t, c.UseNative(SupportYes, key, schema), "a cached verdict does not override known-supported")

	var nilCache *SupportCache
	require.True(t, nilCache.UseNative(SupportUnknown, key, schema), "a nil cache probes when support is unknown")
}
