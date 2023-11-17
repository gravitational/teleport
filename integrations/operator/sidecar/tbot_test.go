/*
Copyright 2023 Gravitational, Inc.

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

package sidecar

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

type mockClientBuilder struct {
	counter atomic.Int32
}

func (m *mockClientBuilder) buildClient(_ context.Context) (*SyncClient, error) {
	m.counter.Add(1)
	return NewSyncClient(nil), nil
}

func (m *mockClientBuilder) countClientBuild() int {
	count := m.counter.Load()
	count32 := int(count)
	return count32
}

func TestBot_GetClient(t *testing.T) {
	ctx := context.Background()

	cert1 := []byte("cert1")
	cert2 := []byte("cert2")

	tests := []struct {
		name                 string
		running              bool
		currentCert          []byte
		cachedCert           []byte
		cachedClient         *SyncClient
		expectNewClientBuild require.BoolAssertionFunc
		assertError          require.ErrorAssertionFunc
	}{
		{
			name:                 "not started",
			running:              false,
			currentCert:          nil,
			cachedCert:           nil,
			cachedClient:         nil,
			expectNewClientBuild: require.False,
			assertError:          require.Error,
		},
		{
			name:                 "no cert yet",
			running:              true,
			currentCert:          nil,
			cachedCert:           nil,
			cachedClient:         nil,
			expectNewClientBuild: require.False,
			assertError:          require.Error,
		},
		{
			name:                 "cert but no cache",
			running:              true,
			currentCert:          cert1,
			cachedCert:           nil,
			cachedClient:         nil,
			expectNewClientBuild: require.True,
			assertError:          require.NoError,
		},
		{
			name:                 "cert and fresh cache",
			running:              true,
			currentCert:          cert1,
			cachedCert:           cert1,
			cachedClient:         NewSyncClient(nil),
			expectNewClientBuild: require.False,
			assertError:          require.NoError,
		},
		{
			name:                 "cert and stale cache",
			running:              true,
			currentCert:          cert2,
			cachedCert:           cert1,
			cachedClient:         NewSyncClient(nil),
			expectNewClientBuild: require.True,
			assertError:          require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := mockClientBuilder{}
			destination := &config.DestinationMemory{}
			require.NoError(t, destination.CheckAndSetDefaults())
			require.NoError(t, destination.Write(ctx, identity.TLSCertKey, tt.currentCert))
			b := &Bot{
				cfg: &config.BotConfig{
					Storage: &config.StorageConfig{
						Destination: destination,
					},
					Outputs: []config.Output{
						&config.IdentityOutput{
							Destination: destination,
						},
					},
				},
				running:       tt.running,
				cachedCert:    tt.cachedCert,
				cachedClient:  tt.cachedClient,
				clientBuilder: mock.buildClient,
			}
			_, _, err := b.GetSyncClient(ctx)
			tt.assertError(t, err)
			tt.expectNewClientBuild(t, mock.countClientBuild() != 0)
		})
	}
}
