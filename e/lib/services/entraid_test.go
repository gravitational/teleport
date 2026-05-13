package services

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/entraid"
)

func TestEntraIntervals(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	testCases := []struct {
		name     string
		in       *types.PluginEntraIDSyncIntervals
		expected entraid.SyncIntervals
	}{
		{
			name: "nil intervals",
			in:   nil,
			expected: entraid.SyncIntervals{
				Delta: 0,
				Full:  entraid.DefaultFullSyncInterval,
			},
		},
		{
			name: "zero intervals (delta=0,full=0)",
			in: &types.PluginEntraIDSyncIntervals{
				Delta: "0",
				Full:  "0",
			},
			expected: entraid.SyncIntervals{
				Delta: 0,
				Full:  entraid.DefaultFullSyncInterval,
			},
		},
		{
			name: "delta only",
			in: &types.PluginEntraIDSyncIntervals{
				Delta: "2m",
				Full:  "0",
			},
			expected: entraid.SyncIntervals{
				Delta: 2 * time.Minute,
				Full:  0,
			},
		},
		{
			name: "full only",
			in: &types.PluginEntraIDSyncIntervals{
				Delta: "0",
				Full:  "5m",
			},
			expected: entraid.SyncIntervals{
				Delta: 0,
				Full:  5 * time.Minute,
			},
		},
		{
			name: "delta and full",
			in: &types.PluginEntraIDSyncIntervals{
				Delta: "2m",
				Full:  "1h",
			},
			expected: entraid.SyncIntervals{
				Delta: 2 * time.Minute,
				Full:  1 * time.Hour,
			},
		},
		{
			name: "delta greater than full",
			in: &types.PluginEntraIDSyncIntervals{
				Delta: "5m",
				Full:  "2m",
			},
			expected: entraid.SyncIntervals{
				Delta: 0, // delta skipped
				Full:  2 * time.Minute,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := entraSyncIntervals(ctx, tc.in, slog.Default())
			require.Equal(t, tc.expected, got, "entra sync intervals deviated")
		})
	}
}
