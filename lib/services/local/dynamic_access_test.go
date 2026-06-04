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
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

func TestSetAccessRequestState(t *testing.T) {
	service, _ := setupDynamicAccessService(t)
	cases := []struct {
		name         string
		initialState types.RequestState
		updatedState types.RequestState
		wantState    types.RequestState
		wantErr      bool
	}{
		{
			name:         "none to pending",
			initialState: types.RequestState_NONE,
			updatedState: types.RequestState_PENDING,
			wantState:    types.RequestState_PENDING,
		},
		{
			name:         "none to approved",
			initialState: types.RequestState_NONE,
			updatedState: types.RequestState_APPROVED,
			wantState:    types.RequestState_APPROVED,
		},
		{
			name:         "none to denied",
			initialState: types.RequestState_NONE,
			updatedState: types.RequestState_DENIED,
			wantState:    types.RequestState_DENIED,
		},
		{
			name:         "none to promoted",
			initialState: types.RequestState_NONE,
			updatedState: types.RequestState_PROMOTED,
			wantState:    types.RequestState_PROMOTED,
		},
		{
			name:         "pending to approved",
			initialState: types.RequestState_PENDING,
			updatedState: types.RequestState_APPROVED,
			wantState:    types.RequestState_APPROVED,
		},
		{
			name:         "pending to denied",
			initialState: types.RequestState_PENDING,
			updatedState: types.RequestState_DENIED,
			wantState:    types.RequestState_DENIED,
		},
		{
			name:         "pending to promoted",
			initialState: types.RequestState_PENDING,
			updatedState: types.RequestState_PROMOTED,
			wantState:    types.RequestState_PROMOTED,
		},
		{
			name:         "approved to pending",
			initialState: types.RequestState_APPROVED,
			updatedState: types.RequestState_PENDING,
			wantState:    types.RequestState_APPROVED,
			wantErr:      true,
		},
		{
			name:         "approved to approved",
			initialState: types.RequestState_APPROVED,
			updatedState: types.RequestState_APPROVED,
			wantState:    types.RequestState_APPROVED,
			wantErr:      true,
		},
		{
			name:         "approved to denied",
			initialState: types.RequestState_APPROVED,
			updatedState: types.RequestState_DENIED,
			wantState:    types.RequestState_APPROVED,
			wantErr:      true,
		},
		{
			name:         "approved to promoted",
			initialState: types.RequestState_APPROVED,
			updatedState: types.RequestState_PROMOTED,
			wantState:    types.RequestState_APPROVED,
			wantErr:      true,
		},
		{
			name:         "denied to denied",
			initialState: types.RequestState_DENIED,
			updatedState: types.RequestState_DENIED,
			wantState:    types.RequestState_DENIED,
			wantErr:      true,
		},
		{
			name:         "promoted to pending",
			initialState: types.RequestState_PROMOTED,
			updatedState: types.RequestState_PENDING,
			wantState:    types.RequestState_PROMOTED,
			wantErr:      true,
		},
		{
			name:         "promoted to approved",
			initialState: types.RequestState_PROMOTED,
			updatedState: types.RequestState_APPROVED,
			wantState:    types.RequestState_PROMOTED,
			wantErr:      true,
		},
		{
			name:         "promoted to denied",
			initialState: types.RequestState_PROMOTED,
			updatedState: types.RequestState_DENIED,
			wantState:    types.RequestState_PROMOTED,
			wantErr:      true,
		},
		{
			name:         "promoted to promoted",
			initialState: types.RequestState_PROMOTED,
			updatedState: types.RequestState_PROMOTED,
			wantState:    types.RequestState_PROMOTED,
			wantErr:      true,
		},
		{
			name:         "none to none",
			initialState: types.RequestState_NONE,
			updatedState: types.RequestState_NONE,
			wantState:    types.RequestState_PENDING,
			wantErr:      true,
		},
		{
			name:         "pending to none",
			initialState: types.RequestState_PENDING,
			updatedState: types.RequestState_NONE,
			wantState:    types.RequestState_PENDING,
			wantErr:      true,
		},
		{
			name:         "pending to pending",
			initialState: types.RequestState_PENDING,
			updatedState: types.RequestState_PENDING,
			wantState:    types.RequestState_PENDING,
		},
		{
			name:         "approved to none",
			initialState: types.RequestState_APPROVED,
			updatedState: types.RequestState_NONE,
			wantState:    types.RequestState_APPROVED,
			wantErr:      true,
		},
		{
			name:         "denied to none",
			initialState: types.RequestState_DENIED,
			updatedState: types.RequestState_NONE,
			wantState:    types.RequestState_DENIED,
			wantErr:      true,
		},
		{
			name:         "denied to pending",
			initialState: types.RequestState_DENIED,
			updatedState: types.RequestState_PENDING,
			wantState:    types.RequestState_DENIED,
			wantErr:      true,
		},
		{
			name:         "denied to approved",
			initialState: types.RequestState_DENIED,
			updatedState: types.RequestState_APPROVED,
			wantState:    types.RequestState_DENIED,
			wantErr:      true,
		},
		{
			name:         "denied to promoted",
			initialState: types.RequestState_DENIED,
			updatedState: types.RequestState_PROMOTED,
			wantState:    types.RequestState_DENIED,
			wantErr:      true,
		},
		{
			name:         "promoted to none",
			initialState: types.RequestState_PROMOTED,
			updatedState: types.RequestState_NONE,
			wantState:    types.RequestState_PROMOTED,
			wantErr:      true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req, err := types.NewAccessRequest(uuid.NewString(), "alice", "test_role_1")
			require.NoError(t, err)
			require.NoError(t, req.SetState(test.initialState))

			err = service.CreateAccessRequest(t.Context(), req)
			require.NoError(t, err)

			final, err := service.SetAccessRequestState(t.Context(), types.AccessRequestUpdate{
				RequestID: req.GetName(),
				State:     test.updatedState,
			})
			if test.wantErr {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				final, err = service.GetAccessRequest(t.Context(), req.GetName())
				require.NoError(t, err)
				require.Equal(t, test.wantState, final.GetState())
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantState, final.GetState())
				final, err = service.GetAccessRequest(t.Context(), req.GetName())
				require.NoError(t, err)
				require.Equal(t, test.wantState, final.GetState())
			}
		})
	}
}

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
