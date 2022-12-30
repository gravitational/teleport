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

package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"
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
	clientMap := NewClientMap(mockNewClientFunc)

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
}
