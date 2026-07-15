/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package discovery

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type fakeSyntheticAccessPoint struct {
	authclient.DiscoveryAccessPoint

	mu      sync.Mutex
	err     error
	upserts chan *discoveryconfig.DiscoveryConfig
}

func (f *fakeSyntheticAccessPoint) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeSyntheticAccessPoint) UpsertDiscoveryConfig(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	f.mu.Lock()
	err := f.err
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	f.upserts <- dc.Clone()
	return dc, nil
}

func TestSyntheticDiscoveryConfigPublisher(t *testing.T) {
	t.Parallel()

	const serverID = "00000000-0000-0000-0000-000000000001"

	clock := clockwork.NewFakeClock()
	accessPoint := &fakeSyntheticAccessPoint{
		upserts: make(chan *discoveryconfig.DiscoveryConfig, 16),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := &Server{
		Config: &Config{
			ServerID:       serverID,
			DiscoveryGroup: "prod",
			Matchers: Matchers{
				AWS: []types.AWSMatcher{
					{Types: []string{"ec2"}, Regions: []string{"us-east-1"}},
				},
			},
			AccessPoint: accessPoint,
			Log:         logtest.NewLogger(),
			clock:       clock,
			jitter:      func(d time.Duration) time.Duration { return d },
		},
		ctx: ctx,
	}

	waitForUpsert := func(t *testing.T) *discoveryconfig.DiscoveryConfig {
		t.Helper()
		select {
		case dc := <-accessPoint.upserts:
			return dc
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for a synthetic discovery config upsert")
			return nil
		}
	}

	s.startSyntheticDiscoveryConfigPublisher()

	// The initial publication happens immediately.
	dc := waitForUpsert(t)
	require.Equal(t, discoveryconfig.SyntheticName(serverID), dc.GetName())
	require.True(t, dc.IsSynthetic())
	require.Equal(t, dc.GetName(), dc.GetDiscoveryGroup())
	require.Equal(t, types.OriginConfigFile, dc.Origin())
	require.Equal(t, "prod", dc.GetAllLabels()[types.TeleportInternalDiscoveryGroupName])
	require.Equal(t, clock.Now().UTC().Add(syntheticDiscoveryConfigTTL), dc.Expiry())
	require.True(t, s.syntheticDiscoveryConfigPublished.Load())

	// The keep-alive refresh re-upserts the same content with a new expiry.
	require.NoError(t, clock.BlockUntilContext(ctx, 1))
	clock.Advance(syntheticDiscoveryConfigKeepAliveInterval)
	refreshed := waitForUpsert(t)
	require.Equal(t, dc.GetName(), refreshed.GetName())
	require.Equal(t, clock.Now().UTC().Add(syntheticDiscoveryConfigTTL), refreshed.Expiry())

	// An access denied error (e.g. an older Auth server) marks the synthetic
	// config as unpublished and keeps retrying at the keep-alive cadence.
	accessPoint.setError(trace.AccessDenied("not allowed"))
	require.NoError(t, clock.BlockUntilContext(ctx, 1))
	clock.Advance(syntheticDiscoveryConfigKeepAliveInterval)
	require.NoError(t, clock.BlockUntilContext(ctx, 1))
	require.False(t, s.syntheticDiscoveryConfigPublished.Load())

	// Recovery: the next successful upsert marks it published again.
	accessPoint.setError(nil)
	clock.Advance(syntheticDiscoveryConfigKeepAliveInterval)
	waitForUpsert(t)
	require.True(t, s.syntheticDiscoveryConfigPublished.Load())

	// A name conflict (a user-created resource occupying the synthetic name)
	// also marks the config as unpublished, and keeps retrying at the
	// keep-alive cadence.
	accessPoint.setError(trace.AlreadyExists("name is taken"))
	require.NoError(t, clock.BlockUntilContext(ctx, 1))
	clock.Advance(syntheticDiscoveryConfigKeepAliveInterval)
	require.NoError(t, clock.BlockUntilContext(ctx, 1))
	require.False(t, s.syntheticDiscoveryConfigPublished.Load())

	// Recovery after the conflicting resource is deleted.
	accessPoint.setError(nil)
	clock.Advance(syntheticDiscoveryConfigKeepAliveInterval)
	waitForUpsert(t)
	require.True(t, s.syntheticDiscoveryConfigPublished.Load())
}

func TestSyntheticDiscoveryConfigPublisherNoStaticMatchers(t *testing.T) {
	t.Parallel()

	accessPoint := &fakeSyntheticAccessPoint{
		upserts: make(chan *discoveryconfig.DiscoveryConfig, 1),
	}

	s := &Server{
		Config: &Config{
			ServerID:       "00000000-0000-0000-0000-000000000002",
			DiscoveryGroup: "prod",
			AccessPoint:    accessPoint,
			Log:            logtest.NewLogger(),
			clock:          clockwork.NewFakeClock(),
			jitter:         func(d time.Duration) time.Duration { return d },
		},
		ctx: context.Background(),
	}

	// Without static matchers there is nothing to publish.
	s.startSyntheticDiscoveryConfigPublisher()
	select {
	case <-accessPoint.upserts:
		t.Fatal("no synthetic discovery config should be published without static matchers")
	case <-time.After(100 * time.Millisecond):
	}
}
