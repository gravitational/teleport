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

package oidc

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
		path     string
		expected string
	}{
		{
			name:     "valid host:port",
			addr:     "127.0.0.1.nip.io:3080",
			expected: "https://127.0.0.1.nip.io:3080",
		},
		{
			name:     "valid host:port with path",
			addr:     "127.0.0.1.nip.io:3080",
			path:     "/workload-identity",
			expected: "https://127.0.0.1.nip.io:3080/workload-identity",
		},
		{
			name:     "valid ip:port",
			addr:     "127.0.0.1:3080",
			expected: "https://127.0.0.1:3080",
		},
		{
			name:     "valid ip:port with path",
			addr:     "127.0.0.1:3080",
			path:     "/workload-identity",
			expected: "https://127.0.0.1:3080/workload-identity",
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
		{
			name:     "only host with path",
			addr:     "localhost",
			path:     "/workload-identity",
			expected: "https://localhost/workload-identity",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IssuerFromPublicAddress(tt.addr, tt.path)
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
		path           string
		mockProxies    []types.Server
		mockErr        error
		checkErr       require.ErrorAssertionFunc
		expectedIssuer string
	}{
		{
			name: "valid",
			mockProxies: []types.Server{
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddrs: []string{"127.0.0.1.nip.io"},
				}},
			},
			expectedIssuer: "https://127.0.0.1.nip.io",
		},
		{
			name: "valid with subpath",
			path: "/workload-identity",
			mockProxies: []types.Server{
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddrs: []string{"127.0.0.1.nip.io"},
				}},
			},
			expectedIssuer: "https://127.0.0.1.nip.io/workload-identity",
		},
		{
			name: "only the second server has a valid public address",
			mockProxies: []types.Server{
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddrs: []string{""},
				}},
				&types.ServerV2{Spec: types.ServerSpecV2{
					PublicAddrs: []string{"127.0.0.1.nip.io"},
				}},
			},
			expectedIssuer: "https://127.0.0.1.nip.io",
		},
		{
			name:     "api returns not found",
			mockErr:  &trace.NotFoundError{},
			checkErr: notFoundCheck,
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
			issuer, err := IssuerForCluster(ctx, clt, tt.path)
			if tt.checkErr != nil {
				tt.checkErr(t, err)
			}
			if tt.expectedIssuer != "" {
				require.Equal(t, tt.expectedIssuer, issuer)
			}
		})
	}
}

func badParameterCheck(t require.TestingT, err error, msgAndArgs ...any) {
	require.True(t, trace.IsBadParameter(err), `expected "bad parameter", but got %v`, err)
}

func notFoundCheck(t require.TestingT, err error, msgAndArgs ...any) {
	require.True(t, trace.IsNotFound(err), `expected "not found", but got %v`, err)
}
