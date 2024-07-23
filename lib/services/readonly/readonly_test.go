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

package readonly

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	protobuf "google.golang.org/protobuf/proto"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

type testUpstream struct {
	auth                types.AuthPreference
	networking          types.ClusterNetworkingConfig
	recording           types.SessionRecordingConfig
	accessGraphSettings *clusterconfigpb.AccessGraphSettings
}

func (u *testUpstream) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return u.auth.Clone(), nil
}

func (u *testUpstream) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	return u.networking.Clone(), nil
}

func (u *testUpstream) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return u.recording.Clone(), nil
}

func (u *testUpstream) GetAccessGraphSettings(ctx context.Context) (*clusterconfigpb.AccessGraphSettings, error) {
	return protobuf.Clone(u.accessGraphSettings).(*clusterconfigpb.AccessGraphSettings), nil
}

// TestAuthPreference tests the GetReadOnlyAuthPreference method and verifies the read-only protections
// on the returned resource.
func TestAuthPreference(t *testing.T) {
	upstreamCfg, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{})
	require.NoError(t, err)

	// Create a new cache instance.
	cache, err := NewCache(CacheConfig{
		Upstream: &testUpstream{
			auth: upstreamCfg,
		},
		TTL: time.Hour,
	})
	require.NoError(t, err)

	// Get the auth preference resource.
	authPref, err := cache.GetReadOnlyAuthPreference(context.Background())
	require.NoError(t, err)

	// Verify that the auth preference resource cannot be cast back to a write-supporting interface.
	_, ok := authPref.(types.AuthPreference)
	require.False(t, ok)

	authPref2, err := cache.GetReadOnlyAuthPreference(context.Background())
	require.NoError(t, err)

	// verify pointer equality (i.e. that subsequent reads return the same shared resource).
	require.True(t, pointersEqual(authPref, authPref2))
}

func TestClusterNetworkingConfig(t *testing.T) {
	// Create a new cache instance.
	cache, err := NewCache(CacheConfig{
		Upstream: &testUpstream{
			networking: types.DefaultClusterNetworkingConfig(),
		},
		TTL: time.Hour,
	})
	require.NoError(t, err)

	// Get the cluster networking config resource.
	networking, err := cache.GetReadOnlyClusterNetworkingConfig(context.Background())
	require.NoError(t, err)

	// Verify that the cluster networking config resource cannot be cast back to a write-supporting interface.
	_, ok := networking.(types.ClusterNetworkingConfig)
	require.False(t, ok)

	networking2, err := cache.GetReadOnlyClusterNetworkingConfig(context.Background())
	require.NoError(t, err)

	// verify pointer equality (i.e. that subsequent reads return the same shared resource).
	require.True(t, pointersEqual(networking, networking2))
}

func TestSessionRecordingConfig(t *testing.T) {
	// Create a new cache instance.
	cache, err := NewCache(CacheConfig{
		Upstream: &testUpstream{
			recording: types.DefaultSessionRecordingConfig(),
		},
		TTL: time.Hour,
	})
	require.NoError(t, err)

	// Get the session recording config resource.
	recording, err := cache.GetReadOnlySessionRecordingConfig(context.Background())
	require.NoError(t, err)

	// Verify that the session recording config resource cannot be cast back to a write-supporting interface.
	_, ok := recording.(types.SessionRecordingConfig)
	require.False(t, ok)

	recording2, err := cache.GetReadOnlySessionRecordingConfig(context.Background())
	require.NoError(t, err)

	// verify pointer equality (i.e. that subsequent reads return the same shared resource).
	require.True(t, pointersEqual(recording, recording2))
}

// TestCloneBreaksEquality tests that cloning a resource breaks equality with the original resource
// (this is a sanity-check to make sure that the other tests in this package work since they rely upon
// cloned resources being distinct from the original in terms of interface equality).
func TestCloneBreaksEquality(t *testing.T) {
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{})
	require.NoError(t, err)
	require.False(t, pointersEqual(authPref, authPref.Clone()))

	networking := types.DefaultClusterNetworkingConfig()
	require.False(t, pointersEqual(networking, networking.Clone()))

	recording := types.DefaultSessionRecordingConfig()
	require.False(t, pointersEqual(recording, recording.Clone()))
}

func TestAccessGraphSettings(t *testing.T) {
	// Create a new cache instance.
	cache, err := NewCache(CacheConfig{
		Upstream: &testUpstream{
			accessGraphSettings: &clusterconfigpb.AccessGraphSettings{
				Kind: types.KindAccessGraphSettings,
				Metadata: &headerv1.Metadata{
					Name: "access-graph-settings",
				},
			},
		},
		TTL: time.Hour,
	})
	require.NoError(t, err)

	// Get the session recording config resource.
	ag, err := cache.GetReadOnlyAccessGraphSettings(context.Background())
	require.NoError(t, err)

	// Verify that the access graph settings resource cannot be cast back to a write-supporting interface.
	// We do this by checking if the resource is of the sealedAccessGraphSettings type, which is a read-only
	// wrapper around the original resource and should not be directly castable back to the original type.
	_, ok := ag.(sealedAccessGraphSettings)
	require.True(t, ok)

	ag2, err := cache.GetReadOnlyAccessGraphSettings(context.Background())
	require.NoError(t, err)

	// verify pointer equality (i.e. that subsequent reads return the same shared resource).
	require.True(t, pointersEqual(ag, ag2))
}

// pointersEqual is a helper function that compares two pointers for equality. used to improve readability
// and avoid incorrect lints.
func pointersEqual(a, b interface{}) bool {
	return a == b
}
