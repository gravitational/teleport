// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.package local

package local

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
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

func Test_DynamicAccessService_range_boundary(t *testing.T) {
	t.Parallel()
	synctest.Test(t, syncTest_DynamicAccessService_range_boundary)
}

func syncTest_DynamicAccessService_range_boundary(t *testing.T) {
	ctx := t.Context()
	service, _ := setupDynamicAccessService(t)

	ongoingRequest := createAccessRequestWithExpiry(t, service, time.Now().Add(1))
	expiredRequest := createAccessRequestWithExpiry(t, service, time.Now().Add(-1))
	// Create some access requests outside the expected key range to validate they are
	// not listed.
	mustUpsertAccessRequestCustomKey(t, service, backend.NewKey("aaa", paramsPrefix), newAccessRequestWithExpiry(t, time.Now().Add(1)))
	mustUpsertAccessRequestCustomKey(t, service, backend.NewKey("aaa", paramsPrefix), newAccessRequestWithExpiry(t, time.Now().Add(-1)))
	mustUpsertAccessRequestCustomKey(t, service, backend.NewKey("zzz", paramsPrefix), newAccessRequestWithExpiry(t, time.Now().Add(1)))
	mustUpsertAccessRequestCustomKey(t, service, backend.NewKey("zzz", paramsPrefix), newAccessRequestWithExpiry(t, time.Now().Add(-1)))

	listAccessRequestsFn := func(ctx context.Context, limit int, pageToken string) ([]*types.AccessRequestV3, string, error) {
		resp, err := service.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{Limit: int32(limit), StartKey: pageToken})
		return resp.GetAccessRequests(), resp.GetNextKey(), err
	}
	allRequests, err := stream.Collect(clientutils.ResourcesWithPageSize(ctx, listAccessRequestsFn, 1))
	require.NoError(t, err)
	require.Len(t, allRequests, 2)
	require.ElementsMatch(t, resourceNames(ongoingRequest, expiredRequest), resourceNames(allRequests...))

	expiredRequests, err := stream.Collect(clientutils.ResourcesWithPageSize(ctx, service.ListExpiredAccessRequests, 1))
	require.NoError(t, err)
	require.Len(t, expiredRequests, 1)
	require.Equal(t, expiredRequest.GetName(), expiredRequests[0].GetName())
}

func setupDynamicAccessService(t *testing.T) (*DynamicAccessService, *memory.Memory) {
	t.Helper()
	ctx := t.Context()

	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	return NewDynamicAccessService(backend.NewSanitizer(mem)), mem
}

func newAccessRequestWithExpiry(t *testing.T, expiry time.Time) types.AccessRequest {
	t.Helper()

	req, err := types.NewAccessRequest(uuid.NewString(), "alice", "test_role_1")
	require.NoError(t, err)
	req.SetExpiry(expiry)

	return req
}

func createAccessRequestWithExpiry(t *testing.T, service *DynamicAccessService, expiry time.Time) types.AccessRequest {
	t.Helper()
	ctx := t.Context()

	req, err := service.CreateAccessRequestV2(ctx, newAccessRequestWithExpiry(t, expiry))
	require.NoError(t, err)

	return req
}

func mustUpsertAccessRequestCustomKey(t *testing.T, service *DynamicAccessService, key backend.Key, req types.AccessRequest) types.AccessRequest {
	t.Helper()
	ctx := t.Context()

	err := services.ValidateAccessRequest(req)
	require.NoError(t, err)

	item, err := itemFromAccessRequest(req)
	require.NoError(t, err)
	item.Key = key

	lease, err := service.Put(ctx, item)
	require.NoError(t, err)

	req.SetRevision(lease.Revision)
	return req
}

func resourceNames[T types.Resource](resources ...T) []string {
	var names []string
	for _, r := range resources {
		names = append(names, r.GetName())
	}
	return names
}
