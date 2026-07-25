// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestTeleportProcess_NewLocalCache verifies that the Cache returned by
// NewLocalCache is correctly configured.
func TestTeleportProcess_NewLocalCache(t *testing.T) {
	t.Parallel()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Proxy.Enabled = false
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		if assert.NoError(t, process.Close()) {
			assert.NoError(t, process.Wait())
		}
	})

	authClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	// Wrap client so we can fake Enterprise endpoints.
	cacheClient := &fakeLocalCacheClient{
		ClientI: authClient,
	}

	// Prepare local cache.
	cacheSetup := func(cfg cache.Config) cache.Config {
		cfg.Unstarted = true
		cfg.Watches = []types.WatchKind{
			{Kind: types.KindCertAuthorityOverride},
		}
		return cfg
	}
	localCache, err := process.NewLocalCache(cacheClient, cacheSetup, []string{"test-local-cache"})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, localCache.Close()) })

	// Exercise the cache. Add a test for your resource below.
	tests := []struct {
		name     string
		exercise func(t *testing.T, c *cache.Cache)
	}{
		{
			name: "SubCA",
			exercise: func(t *testing.T, c *cache.Cache) {
				got, err := c.GetCertAuthorityOverride(t.Context(), types.CertAuthorityOverrideID{})
				assert.NoError(t, err, "GetCertAuthorityOverride errored")
				assert.NotNil(t, got, "GetCertAuthorityOverride: unexpected response")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.exercise(t, localCache)
		})
	}
}

// TestTeleportProcess_CacheHealthReporter verifies that a health reporter gets
// added into a TeleportProcess and multiple caches share the same health
// reporter. This is because the test in lib/cache/health_test.go is a model of
// the system and does not check if a health reporter is actually added and
// shared in the real processes.
func TestTeleportProcess_CacheHealthReporter(t *testing.T) {
	t.Parallel()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Proxy.Enabled = false
			cfg.CachePolicy.Enabled = true
		}))
	require.NoError(t, err)
	t.Cleanup(func() {
		if assert.NoError(t, process.Close()) {
			assert.NoError(t, process.Wait())
		}
	})

	authCache, ok := process.GetAuthServer().Cache.(*cache.Cache)
	require.True(t, ok)

	authClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	localCache, err := process.NewLocalCache(
		authClient,
		func(cfg cache.Config) cache.Config {
			cfg.Unstarted = true
			return cfg
		},
		[]string{"test-local-cache"},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, localCache.Close())
	})

	require.Same(t,
		authCache.Config.HealthReporter,
		localCache.Config.HealthReporter)
}

type fakeLocalCacheClient struct {
	authclient.ClientI
}

func (s *fakeLocalCacheClient) GetCertAuthorityOverride(context.Context, types.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error) {
	return &subcav1.CertAuthorityOverride{}, nil
}
