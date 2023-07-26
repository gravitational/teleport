package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestAccessRequestMonthlyLimit_UnlimitedByDefault(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(
		time.Date(2023, 07, 15, 1, 2, 3, 0, time.UTC))

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service := NewDynamicAccessService(mem)

	for i := 0; i < 100; i++ {
		req, err := types.NewAccessRequest(uuid.New().String(), "alice", "a-role")
		require.NoError(t, err)
		req.SetCreationTime(clock.Now())
		err = service.CreateAccessRequest(ctx, req)
		require.NoError(t, err)
	}
}

func TestAccessRequestMonthlyLimit(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			IsUsageBasedBilling: true,
			AccessRequests: modules.AccessRequestsFeature{
				Enabled:             true,
				MonthlyRequestLimit: 3,
			},
		},
	})

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
	june := clock.Now().AddDate(0, -1, 0)
	july := clock.Now()
	august := clock.Now().AddDate(0, 1, 0)

	requestDates := []time.Time{
		// Three access requests in June
		june.AddDate(0, 0, 1),
		june.AddDate(0, 0, 2),
		june.AddDate(0, 0, 3),

		// Two access requests in July
		july.AddDate(0, 0, -2),
		july.AddDate(0, 0, -1),

		// Three access requests in August
		august.AddDate(0, 0, -3),
		august.AddDate(0, 0, -2),
		august.AddDate(0, 0, -1),
	}

	for _, date := range requestDates {
		req, err := types.NewAccessRequest(uuid.New().String(), "alice", "a-role")
		require.NoError(t, err)
		req.SetCreationTime(date)
		err = service.CreateAccessRequest(ctx, req)
		require.NoError(t, err)
	}

	julyRequest, err := types.NewAccessRequest(uuid.New().String(), "alice", "a-role")
	require.NoError(t, err)
	julyRequest.SetCreationTime(clock.Now())
	require.NoError(t, service.CreateAccessRequest(ctx, julyRequest))

	clock.Advance(31 * 24 * time.Hour)
	augustRequest, err := types.NewAccessRequest(uuid.New().String(), "alice", "a-role")
	require.NoError(t, err)
	augustRequest.SetCreationTime(clock.Now())
	require.Error(t, service.CreateAccessRequest(ctx, augustRequest))
}
