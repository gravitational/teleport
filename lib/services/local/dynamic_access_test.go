package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestGetAccessRequestMonthlyUsage(t *testing.T) {
	ctx := context.Background()
	// Create a clock in the middle of the month for easy manipulation
	clock := clockwork.NewFakeClockAt(
		time.Date(2023, 07, 15, 1, 2, 3, 0, time.UTC))

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service := NewDynamicAccessService(mem)

	// Create initial data
	requestDates := []time.Time{
		clock.Now().AddDate(0, -1, 0), // a month ago, should not be counted
		clock.Now().AddDate(0, 0, -2), // two days ago, should be counted
		clock.Now(),                   // "now", should be counted
		clock.Now().AddDate(0, 1, 0),  // a month from now, should not be counted
	}

	for _, date := range requestDates {
		req, err := types.NewAccessRequest(uuid.New().String(), "alice", "a-role")
		require.NoError(t, err)
		req.SetCreationTime(date)
		err = service.CreateAccessRequest(ctx, req)
		require.NoError(t, err)
	}

	usage, err := service.getAccessRequestMonthlyUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, usage)
}
