/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestClientMap(t *testing.T) {
	t.Parallel()

	mockNewClientFunc := func(subscription string, cred azcore.TokenCredential, opts *arm.ClientOptions) (CacheForRedisClient, error) {
		if subscription == "good-sub" {
			return NewRedisClientByAPI(nil), nil
		}
		return nil, trace.BadParameter("failed to create")
	}
	clock := clockwork.NewFakeClock()
	clientMap, err := NewClientMap(mockNewClientFunc, withClock(clock))
	require.NoError(t, err)

	// Note that some test cases (e.g. "get from cache") depend on previous
	// test cases. Thus running in sequence.
	t.Run("get credentials failed", func(t *testing.T) {
		client, err := clientMap.Get("some-sub", func() (azcore.TokenCredential, error) {
			return nil, trace.AccessDenied("failed to get credentials")
		})
		require.ErrorIs(t, err, trace.AccessDenied("failed to get credentials"))
		require.Nil(t, client)
	})

	t.Run("create client failed", func(t *testing.T) {
		client, err := clientMap.Get("bad-sub", func() (azcore.TokenCredential, error) {
			return nil, nil
		})
		require.ErrorIs(t, err, trace.BadParameter("failed to create"))
		require.Nil(t, client)
	})

	t.Run("create client succeed", func(t *testing.T) {
		client, err := clientMap.Get("good-sub", func() (azcore.TokenCredential, error) {
			return nil, nil
		})
		require.NoError(t, err)
		require.NotNil(t, client)
		require.IsType(t, NewRedisClientByAPI(nil), client)
	})

	t.Run("get from cache", func(t *testing.T) {
		// Return an error for getCredentials but it shouldn't even be called
		// as the client is returned from existing cache.
		client, err := clientMap.Get("good-sub", func() (azcore.TokenCredential, error) {
			return nil, trace.AccessDenied("failed to get credentials")
		})
		require.NoError(t, err)
		require.NotNil(t, client)
		require.IsType(t, NewRedisClientByAPI(nil), client)
	})

	t.Run("expire from cache", func(t *testing.T) {
		oldClient, err := clientMap.Get("good-sub", func() (azcore.TokenCredential, error) {
			return nil, nil
		})
		require.NoError(t, err)
		require.NotNil(t, oldClient)

		clock.Advance(2 * clientExpireTime)
		newClient, err := clientMap.Get("good-sub", func() (azcore.TokenCredential, error) {
			return nil, nil
		})
		require.NoError(t, err)
		require.NotNil(t, newClient)
		require.NotSame(t, oldClient, newClient)
	})

}
