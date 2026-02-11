package local

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func Test_DynamicAccessService_ListExpiredAccessRequests(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		service, _ := setupDynamicAccessService(t)

		// create 2 non-expired, 100 expired, 2 non-expired, 100 expired, 2 non-expired
		// 206 in total
		for range 2 {
			createAccessRequestWithExpiry(t, service, time.Now().Add(1*time.Hour))
		}
		for range 100 {
			createAccessRequestWithExpiry(t, service, time.Now().Add(-1*time.Hour))
		}
		for range 2 {
			createAccessRequestWithExpiry(t, service, time.Now().Add(1*time.Hour))
		}
		for range 100 {
			createAccessRequestWithExpiry(t, service, time.Now().Add(-1*time.Hour))
		}
		for range 2 {
			createAccessRequestWithExpiry(t, service, time.Now().Add(1*time.Hour))
		}

		// List all.
		requests, err := stream.Collect(clientutils.Resources(ctx, service.ListExpiredAccessRequests))
		require.NoError(t, err)
		require.Len(t, requests, 200)

		// List all with an arbitrary page size.
		requests, err = stream.Collect(clientutils.ResourcesWithPageSize(ctx, service.ListExpiredAccessRequests, 27))
		require.NoError(t, err)
		require.Len(t, requests, 200)

		// List all with with a page size of 1.
		requests, err = stream.Collect(clientutils.ResourcesWithPageSize(ctx, service.ListExpiredAccessRequests, 1))
		require.NoError(t, err)
		require.Len(t, requests, 200)

		// List single page.
		requests, nextToken, err := service.ListExpiredAccessRequests(ctx, 83, "")
		require.NoError(t, err)
		require.NotEmpty(t, nextToken)
		require.Len(t, requests, 83)

		// Expire all.
		time.Sleep(1*time.Hour + 1*time.Second)
		synctest.Wait()

		// List all expecting to have all listed as expired.
		requests, err = stream.Collect(clientutils.Resources(ctx, service.ListExpiredAccessRequests))
		require.NoError(t, err)
		require.Len(t, requests, 206)
	})
}

func setupDynamicAccessService(t *testing.T) (*DynamicAccessService, *memory.Memory) {
	t.Helper()
	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	return NewDynamicAccessService(backend.NewSanitizer(mem)), mem
}

func createAccessRequestWithExpiry(t *testing.T, service *DynamicAccessService, expiry time.Time) types.AccessRequest {
	t.Helper()
	ctx := t.Context()

	req, err := types.NewAccessRequest(uuid.NewString(), "alice", "test_role_1")
	require.NoError(t, err)
	req.SetExpiry(expiry)

	req, err = service.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)

	return req
}
