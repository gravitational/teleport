/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type accessRequestServices struct {
	types.Events
	services.DynamicAccessExt
}

func newAccessRequestPack(t *testing.T) (accessRequestServices, *services.AccessRequestCache) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svcs := accessRequestServices{
		Events:           local.NewEventsService(bk),
		DynamicAccessExt: local.NewDynamicAccessService(bk),
	}

	cache, err := services.NewAccessRequestCache(services.AccessRequestCacheConfig{
		Events: svcs,
		Getter: svcs,
	})
	require.NoError(t, err)

	return svcs, cache
}

// TestAccessRequestCacheBasics verifies the basic expected behaviors of the access request cache,
// including correct sorting and handling of put/delete events.
func TestAccessRequestCacheBasics(t *testing.T) {
	t.Parallel()

	svcs, cache := newAccessRequestPack(t)
	defer cache.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// describe a set of basic test resources that we can use to
	// verify various sort scenarios (request are inserted with
	// creation times 1 second apart, according to the order in
	// which they are defined).
	rrs := []struct {
		name  string
		id    string
		state types.RequestState
	}{
		{
			id:    "00000000-0000-0000-0000-000000000005",
			state: types.RequestState_PENDING,
			name:  "bob",
		},
		{
			id:    "00000000-0000-0000-0000-000000000004",
			state: types.RequestState_APPROVED,
			name:  "bob",
		},
		{
			id:    "00000000-0000-0000-0000-000000000003",
			state: types.RequestState_DENIED,
			name:  "alice",
		},
		{
			id:    "00000000-0000-0000-0000-000000000002",
			state: types.RequestState_APPROVED,
			name:  "alice",
		},
		{
			id:    "00000000-0000-0000-0000-000000000001",
			state: types.RequestState_PENDING,
			name:  "jan",
		},
	}

	tts := []struct {
		Sort       proto.AccessRequestSort
		Descending bool
		Expect     []string
	}{
		{
			Sort:       proto.AccessRequestSort_DEFAULT,
			Descending: false,
			Expect: []string{
				"00000000-0000-0000-0000-000000000001",
				"00000000-0000-0000-0000-000000000002",
				"00000000-0000-0000-0000-000000000003",
				"00000000-0000-0000-0000-000000000004",
				"00000000-0000-0000-0000-000000000005",
			},
		},
		{
			Sort:       proto.AccessRequestSort_DEFAULT,
			Descending: true,
			Expect: []string{
				"00000000-0000-0000-0000-000000000005",
				"00000000-0000-0000-0000-000000000004",
				"00000000-0000-0000-0000-000000000003",
				"00000000-0000-0000-0000-000000000002",
				"00000000-0000-0000-0000-000000000001",
			},
		},
		{
			Sort:       proto.AccessRequestSort_CREATED,
			Descending: false,
			Expect: []string{
				"00000000-0000-0000-0000-000000000005",
				"00000000-0000-0000-0000-000000000004",
				"00000000-0000-0000-0000-000000000003",
				"00000000-0000-0000-0000-000000000002",
				"00000000-0000-0000-0000-000000000001",
			},
		},
		{
			Sort:       proto.AccessRequestSort_CREATED,
			Descending: true,
			Expect: []string{
				"00000000-0000-0000-0000-000000000001",
				"00000000-0000-0000-0000-000000000002",
				"00000000-0000-0000-0000-000000000003",
				"00000000-0000-0000-0000-000000000004",
				"00000000-0000-0000-0000-000000000005",
			},
		},
		{
			Sort:       proto.AccessRequestSort_STATE,
			Descending: false,
			Expect: []string{
				"00000000-0000-0000-0000-000000000002", // approved
				"00000000-0000-0000-0000-000000000004", // approved
				"00000000-0000-0000-0000-000000000003", // denied
				"00000000-0000-0000-0000-000000000001", // pending
				"00000000-0000-0000-0000-000000000005", // pending
			},
		},
		{
			Sort:       proto.AccessRequestSort_STATE,
			Descending: true,
			Expect: []string{
				"00000000-0000-0000-0000-000000000005", // pending
				"00000000-0000-0000-0000-000000000001", // pending
				"00000000-0000-0000-0000-000000000003", // denied
				"00000000-0000-0000-0000-000000000004", // approved
				"00000000-0000-0000-0000-000000000002", // approved
			},
		},
		{
			Sort:       proto.AccessRequestSort_USER,
			Descending: true,
			Expect: []string{
				"00000000-0000-0000-0000-000000000001", // jan
				"00000000-0000-0000-0000-000000000005", // bob
				"00000000-0000-0000-0000-000000000004", // bob
				"00000000-0000-0000-0000-000000000003", // alice
				"00000000-0000-0000-0000-000000000002", // alice
			},
		},
		{
			Sort:       proto.AccessRequestSort_USER,
			Descending: false,
			Expect: []string{
				"00000000-0000-0000-0000-000000000002", // alice
				"00000000-0000-0000-0000-000000000003", // alice
				"00000000-0000-0000-0000-000000000004", // bob
				"00000000-0000-0000-0000-000000000005", // bob
				"00000000-0000-0000-0000-000000000001", // jan
			},
		},
	}

	created := time.Now()
	for _, rr := range rrs {
		r, err := types.NewAccessRequest(rr.id, rr.name, "some-role")
		require.NoError(t, err)
		require.NoError(t, r.SetState(rr.state))
		r.SetCreationTime(created.UTC())
		created = created.Add(time.Second)
		_, err = svcs.CreateAccessRequestV2(ctx, r)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			rsp, err := cache.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
				Filter: &types.AccessRequestFilter{
					ID: rr.id,
				},
			})
			return err == nil && len(rsp.AccessRequests) != 0
		}, time.Second*15, time.Millisecond*50)
	}

	timeout := time.After(time.Second * 30)

	for {
		rsp, err := cache.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
			Limit: int32(len(rrs)),
		})
		require.NoError(t, err)
		if len(rsp.AccessRequests) == len(rrs) {
			break
		}

		select {
		case <-timeout:
			require.FailNow(t, "timeout waiting for access request cache to populate")
		case <-time.After(time.Millisecond * 200):
		}
	}

	for _, tt := range tts {

		var nextKey string
		var reqIDs []string
		for {
			rsp, err := cache.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
				StartKey:   nextKey,
				Limit:      3,
				Sort:       tt.Sort,
				Descending: tt.Descending,
			})
			require.NoError(t, err)
			for _, r := range rsp.AccessRequests {
				reqIDs = append(reqIDs, r.GetName())
			}
			nextKey = rsp.NextKey
			if nextKey == "" {
				break
			}
		}

		require.Equal(t, tt.Expect, reqIDs, "index=%s, descending=%v", tt.Sort.String(), tt.Descending)
	}

	// verify that delete events are correctly processed
	timeout = time.After(time.Second * 30)
	for i, rr := range rrs {
		require.NoError(t, svcs.DeleteAccessRequest(ctx, rr.id))
	WaitForReplication:
		for {
			rsp, err := cache.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
				Limit: int32(len(rrs)),
			})
			require.NoError(t, err)
			if len(rsp.AccessRequests) == len(rrs)-(i+1) {
				break WaitForReplication
			}

			select {
			case <-timeout:
				require.FailNow(t, "timeout waiting for cache to to reach expected resource count", "have=%d, want=%d", len(rsp.AccessRequests), len(rrs)-(i+1))
			case <-time.After(time.Millisecond * 200):
			}
		}
	}
}

func TestAccessRequestCacheExpiryFiltering(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		// set backend into mirror mode so that it does not expire items
		// automatically.
		Mirror: true,
	})
	require.NoError(t, err)

	svcs := accessRequestServices{
		Events:           local.NewEventsService(bk),
		DynamicAccessExt: local.NewDynamicAccessService(bk),
	}

	cache, err := services.NewAccessRequestCache(services.AccessRequestCacheConfig{
		Events: svcs,
		Getter: svcs,
	})
	require.NoError(t, err)

	// describe a set of test requests, some of which are expired
	rrs := []struct {
		name    string
		id      string
		expired bool
	}{
		{
			id:      "00000000-0000-0000-0000-000000000005",
			name:    "bob",
			expired: true,
		},
		{
			id:      "00000000-0000-0000-0000-000000000004",
			name:    "bob",
			expired: false,
		},
		{
			id:      "00000000-0000-0000-0000-000000000003",
			name:    "alice",
			expired: true,
		},
		{
			id:      "00000000-0000-0000-0000-000000000002",
			name:    "alice",
			expired: false,
		},
		{
			id:      "00000000-0000-0000-0000-000000000001",
			name:    "jan",
			expired: true,
		},
	}

	// insert test requests into backend, and aggregate the IDs of the subset that
	// are unexpired so that we can check them against cache reads later.
	var unexpiredRequestIDs []string
	for _, rr := range rrs {
		r, err := types.NewAccessRequest(rr.id, rr.name, "some-role")
		require.NoError(t, err)

		if rr.expired {
			r.SetExpiry(time.Now().Add(-time.Minute * 30).UTC())
		} else {
			unexpiredRequestIDs = append(unexpiredRequestIDs, rr.id)
			r.SetExpiry(time.Now().Add(time.Minute * 30).UTC())
		}
		_, err = svcs.CreateAccessRequestV2(ctx, r)
		require.NoError(t, err)
	}

	// verify that once cache replication completes, only the unexpired requests are served
	timeout := time.After(time.Second * 30)
	for {
		rsp, err := cache.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{
			Limit: int32(len(rrs)),
		})
		require.NoError(t, err)

		if len(rsp.AccessRequests) >= len(unexpiredRequestIDs) {
			// once cache is returning the expected number of requests, verify that
			// the set of requests returned is exactly the unexpired subset.
			var returnedRequestIDs []string
			for _, req := range rsp.AccessRequests {
				returnedRequestIDs = append(returnedRequestIDs, req.GetName())
			}

			require.ElementsMatch(t, unexpiredRequestIDs, returnedRequestIDs)
			break
		}

		select {
		case <-timeout:
			require.FailNow(t, "timeout waiting for access request cache to populate")
		case <-time.After(time.Millisecond * 200):
		}
	}
}
