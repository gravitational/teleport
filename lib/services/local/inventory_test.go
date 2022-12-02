/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestInstanceCAS verifies basic expected behavior of instance creation/update.
func TestInstanceCAS(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	defer backend.Close()

	presence := NewPresenceService(backend)

	instance1, err := types.NewInstance(uuid.NewString(), types.InstanceSpecV1{})
	require.NoError(t, err)

	raw1, err := presence.CompareAndSwapInstance(ctx, instance1, nil)
	require.NoError(t, err)

	// verify that "create" style compare and swaps are now rejected
	_, err = presence.CompareAndSwapInstance(ctx, instance1, nil)
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err))

	// get the inserted instance
	instances, err := stream.Collect(presence.GetInstances(ctx, types.InstanceFilter{}))
	require.NoError(t, err)
	require.Len(t, instances, 1)

	// verify that expiry and last_seen are automatically set to expected values.
	exp1 := instances[0].Expiry()
	seen1 := instances[0].GetLastSeen()
	require.False(t, exp1.IsZero())
	require.False(t, seen1.IsZero())
	require.Equal(t, presence.Clock().Now().UTC(), seen1)
	require.Equal(t, seen1.Add(apidefaults.ServerAnnounceTTL), exp1)

	require.True(t, exp1.After(presence.Clock().Now()))
	require.False(t, exp1.After(presence.Clock().Now().Add(apidefaults.ServerAnnounceTTL*2)))

	// update the instance control log
	instance1.AppendControlLog(types.InstanceControlLogEntry{
		Type: "testing",
		ID:   1,
		TTL:  time.Hour * 24,
	})
	instance1.SyncLogAndResourceExpiry(apidefaults.ServerAnnounceTTL)

	// verify expected increase in ttl to accommodate custom log entry TTL (sanity check
	// to differentiate bugs in SyncLogAndResourceExpiry from bugs in presence/backend).
	require.Equal(t, seen1.Add(time.Hour*24), instance1.Expiry())

	// perform normal compare and swap using raw value from previous successful call
	_, err = presence.CompareAndSwapInstance(ctx, instance1, raw1)
	require.NoError(t, err)

	// verify that raw value from previous successful CaS no longer works
	_, err = presence.CompareAndSwapInstance(ctx, instance1, raw1)
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err))

	// load new instance state
	instances2, err := stream.Collect(presence.GetInstances(ctx, types.InstanceFilter{}))
	require.NoError(t, err)
	require.Len(t, instances2, 1)

	// ensure that ttl and log were preserved
	require.Equal(t, seen1.Add(time.Hour*24), instances2[0].Expiry())
	require.Len(t, instances2[0].GetControlLog(), 1)
}

// TestInstanceFiltering tests basic filtering options. A sufficiently large
// instance count is used to ensure that queries span many pages.
func TestInstanceFiltering(t *testing.T) {
	const count = 100_000
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// NOTE: backend must be memory, since parallel subtests are used (makes correct cleanup of
	// filesystem state tricky).
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	defer backend.Close()

	presence := NewPresenceService(backend)

	// store an odd and an even uuid for later use in queries
	var evenID, oddID string

	evenServices := []types.SystemRole{"even"}
	oddServices := []types.SystemRole{"odd"}

	evenVersion := "v2.4.6"
	oddVersion := "v3.5.7"

	allServices := append(evenServices, oddServices...)

	// create a bunch of instances with an even mix of odd/even "services".
	for i := 0; i < count; i++ {
		serverID := uuid.NewString()
		var services []types.SystemRole
		var version string
		if i%2 == 0 {
			services = evenServices
			version = evenVersion
			evenID = serverID
		} else {
			services = oddServices
			version = oddVersion
			oddID = serverID
		}

		instance, err := types.NewInstance(serverID, types.InstanceSpecV1{
			Services: services,
			Version:  version,
		})
		require.NoError(t, err)

		_, err = presence.CompareAndSwapInstance(ctx, instance, nil)
		require.NoError(t, err)
	}

	// check a few simple queries
	tts := []struct {
		filter    types.InstanceFilter
		even, odd int
		desc      string
	}{
		{
			filter: types.InstanceFilter{
				Services: evenServices,
			},
			even: count / 2,
			desc: "all even services",
		},
		{
			filter: types.InstanceFilter{
				ServerID: oddID,
			},
			odd:  1,
			desc: "single-instance direct",
		},
		{
			filter: types.InstanceFilter{
				ServerID: evenID,
				Services: oddServices,
			},
			desc: "non-matching id+service pair",
		},
		{
			filter: types.InstanceFilter{
				ServerID: evenID,
				Services: evenServices,
			},
			even: 1,
			desc: "matching id+service pair",
		},
		{
			filter: types.InstanceFilter{
				Services: allServices,
			},
			even: count / 2,
			odd:  count / 2,
			desc: "all services",
		},
		{
			filter: types.InstanceFilter{
				Version: evenVersion,
			},
			even: count / 2,
			desc: "single version",
		},
	}

	for _, testCase := range tts {
		tt := testCase
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// load instances with given filter
			instances, err := stream.Collect(presence.GetInstances(ctx, tt.filter))
			require.NoError(t, err)

			// aggregate number of s
			var even, odd int
			for _, instance := range instances {
				require.Len(t, instance.GetServices(), 1)
				switch service := instance.GetServices()[0]; service {
				case "even":
					even++
				case "odd":
					odd++
				default:
					t.Fatalf("Unexpected service: %+v", service)
				}
			}

			require.Equal(t, tt.even, even)
			require.Equal(t, tt.odd, odd)
		})
	}
}
