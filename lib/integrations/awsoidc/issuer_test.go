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

package awsoidc

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestIssuerFromPublicAddress(t *testing.T) {
	for _, tt := range []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "valid host:port",
			addr:     "127.0.0.1.nip.io:3080",
			expected: "https://127.0.0.1.nip.io:3080",
		},
		{
			name:     "valid ip:port",
			addr:     "127.0.0.1:3080",
			expected: "https://127.0.0.1:3080",
		},
		{
			name:     "removes 443 port",
			addr:     "https://teleport-local.example.com:443",
			expected: "https://teleport-local.example.com",
		},
		{
			name:     "only host",
			addr:     "localhost",
			expected: "https://localhost",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IssuerFromPublicAddress(tt.addr)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

type mockProxyGetter struct {
	proxies   []types.Server
	returnErr error
}

func (m *mockProxyGetter) GetProxies() ([]types.Server, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	return m.proxies, nil
}

func TestIssuerForCluster(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name           string
		mockProxies    []types.Server
		mockErr        error
		checkErr       require.ErrorAssertionFunc
		expectedIssuer string
	}{
		{
			name: "valid",
			mockProxies: []types.Server{
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddr: "127.0.0.1.nip.io",
				}},
			},
			expectedIssuer: "https://127.0.0.1.nip.io",
		},
		{
			name: "only the second server has a valid public address",
			mockProxies: []types.Server{
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddr: "",
				}},
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddr: "127.0.0.1.nip.io",
				}},
			},
			expectedIssuer: "https://127.0.0.1.nip.io",
		},
		{
			name:     "api returns not found",
			mockErr:  &trace.NotFoundError{},
			checkErr: notFounCheck,
		},
		{
			name:        "api returns an empty list of proxies",
			mockProxies: []types.Server{},
			checkErr:    badParameterCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := &mockProxyGetter{
				proxies:   tt.mockProxies,
				returnErr: tt.mockErr,
			}
			issuer, err := IssuerForCluster(ctx, clt)
			if tt.checkErr != nil {
				tt.checkErr(t, err)
			}
			if tt.expectedIssuer != "" {
				require.Equal(t, tt.expectedIssuer, issuer)
			}
		})
	}
}
