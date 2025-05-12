/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package version

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
)

type mockWebClient struct {
	mock.Mock
}

func (m *mockWebClient) Find() (*webclient.PingResponse, error) {
	args := m.Called()
	return args.Get(0).(*webclient.PingResponse), args.Error(1)
}

func TestProxyVersionClient(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name            string
		pong            *webclient.PingResponse
		pongErr         error
		expectedVersion string
		expectErr       require.ErrorAssertionFunc
	}{
		{
			name: "semver without leading v",
			pong: &webclient.PingResponse{
				AutoUpdate: webclient.AutoUpdateSettings{
					AgentVersion: "1.2.3",
				},
			},
			expectedVersion: "v1.2.3",
			expectErr:       require.NoError,
		},
		{
			name: "semver with leading v",
			pong: &webclient.PingResponse{
				AutoUpdate: webclient.AutoUpdateSettings{
					AgentVersion: "v1.2.3",
				},
			},
			expectedVersion: "v1.2.3",
			expectErr:       require.NoError,
		},
		{
			name: "semver with prerelease and no leading v",
			pong: &webclient.PingResponse{
				AutoUpdate: webclient.AutoUpdateSettings{
					AgentVersion: "1.2.3-dev.bartmoss.1",
				},
			},
			expectedVersion: "v1.2.3-dev.bartmoss.1",
			expectErr:       require.NoError,
		},
		{
			name: "invalid semver",
			pong: &webclient.PingResponse{
				AutoUpdate: webclient.AutoUpdateSettings{
					AgentVersion: "v",
				},
			},
			expectedVersion: "",
			expectErr:       require.Error,
		},
		{
			name:            "empty response",
			pong:            &webclient.PingResponse{},
			expectedVersion: "",
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.NotImplemented("proxy does not seem to implement RFD-184"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: create mock and load fixtures.
			webClient := &mockWebClient{}
			webClient.On("Find").Once().Return(tt.pong, tt.pongErr)

			// Test execution.
			clt := proxyVersionClient{client: webClient}
			v, err := clt.Get(ctx)

			// Test validation.
			tt.expectErr(t, err)
			require.Equal(t, tt.expectedVersion, v)
			webClient.AssertExpectations(t)
		})
	}
}
