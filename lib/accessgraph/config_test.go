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

package accessgraph

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
)

func TestGetAccessGraphSettings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a temporary CA file for testing
	tempDir := t.TempDir()
	caFilePath := filepath.Join(tempDir, "ca.pem")
	caContent := []byte("test-ca-certificate")
	err := os.WriteFile(caFilePath, caContent, 0o600)
	require.NoError(t, err)

	tests := []struct {
		name        string
		config      GetAccessGraphSettingsConfig
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, result AccessGraphConfig)
	}{
		{
			name: "missing ClusterClientGetter",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: true,
			},
			expectError: true,
			errorMsg:    "missing ClusterClientGetter",
		},
		{
			name: "locally enabled with CA file",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: true,
				Addr:           "localhost:50051",
				CA:             caFilePath,
				Insecure:       false,
				CipherSuites:   []uint16{0x1301, 0x1302},
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return nil, nil
				},
			},
			expectError: false,
			validate: func(t *testing.T, result AccessGraphConfig) {
				require.True(t, result.Enabled)
				require.Equal(t, "localhost:50051", result.Addr)
				require.False(t, result.Insecure)
				require.Equal(t, caContent, result.CA)
				require.Equal(t, []uint16{0x1301, 0x1302}, result.CipherSuites)
			},
		},
		{
			name: "locally enabled without CA file",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: true,
				Addr:           "localhost:50051",
				CA:             "",
				Insecure:       true,
				CipherSuites:   []uint16{0x1301},
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return nil, nil
				},
			},
			expectError: false,
			validate: func(t *testing.T, result AccessGraphConfig) {
				require.True(t, result.Enabled)
				require.Equal(t, "localhost:50051", result.Addr)
				require.True(t, result.Insecure)
				require.Nil(t, result.CA)
				require.Equal(t, []uint16{0x1301}, result.CipherSuites)
			},
		},
		{
			name: "locally enabled with invalid CA path",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: true,
				Addr:           "localhost:50051",
				CA:             "/nonexistent/path/ca.pem",
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return nil, nil
				},
			},
			expectError: true,
			errorMsg:    "failed to read access graph CA from path",
		},
		{
			name: "not locally enabled - auth server",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: false,
				IsAuthServer:   true,
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return nil, nil
				},
			},
			expectError: false,
			validate: func(t *testing.T, result AccessGraphConfig) {
				require.False(t, result.Enabled)
				require.Empty(t, result.Addr)
				require.Nil(t, result.CA)
			},
		},
		{
			name: "not locally enabled - cluster client getter fails",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: false,
				IsAuthServer:   false,
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return nil, trace.ConnectionProblem(nil, "connection failed")
				},
			},
			expectError: true,
			errorMsg:    "failed to create cluster config client",
		},
		{
			name: "not locally enabled - GetClusterAccessGraphConfig fails",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: false,
				IsAuthServer:   false,
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return &mockClusterConfigClient{
						getClusterAccessGraphConfigErr: trace.NotFound("config not found"),
					}, nil
				},
			},
			expectError: true,
			errorMsg:    "failed to get access graph settings from auth server",
		},
		{
			name: "not locally enabled - access graph nil in response",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: false,
				IsAuthServer:   false,
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return &mockClusterConfigClient{
						getClusterAccessGraphConfigResp: &clusterconfigv1.GetClusterAccessGraphConfigResponse{},
					}, nil
				},
			},
			expectError: true,
			errorMsg:    "access graph is not enabled in the cluster",
		},
		{
			name: "not locally enabled - access graph disabled in cluster",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: false,
				IsAuthServer:   false,
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return &mockClusterConfigClient{
						getClusterAccessGraphConfigResp: &clusterconfigv1.GetClusterAccessGraphConfigResponse{
							AccessGraph: &clusterconfigv1.AccessGraphConfig{
								Enabled: false,
							},
						},
					}, nil
				},
			},
			expectError: true,
			errorMsg:    "access graph is not enabled in the cluster",
		},
		{
			name: "not locally enabled - successful cluster fetch",
			config: GetAccessGraphSettingsConfig{
				LocallyEnabled: false,
				IsAuthServer:   false,
				CipherSuites:   []uint16{0x1303},
				ClusterClientGetter: func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error) {
					return &mockClusterConfigClient{
						getClusterAccessGraphConfigResp: &clusterconfigv1.GetClusterAccessGraphConfigResponse{
							AccessGraph: &clusterconfigv1.AccessGraphConfig{
								Enabled:  true,
								Address:  "cluster.example.com:443",
								Insecure: false,
								Ca:       []byte("cluster-ca-cert"),
							},
						},
					}, nil
				},
			},
			expectError: false,
			validate: func(t *testing.T, result AccessGraphConfig) {
				require.True(t, result.Enabled)
				require.Equal(t, "cluster.example.com:443", result.Addr)
				require.False(t, result.Insecure)
				require.Equal(t, []byte("cluster-ca-cert"), result.CA)
				require.Equal(t, []uint16{0x1303}, result.CipherSuites)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetAccessGraphSettings(ctx, tt.config)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.ErrorContains(t, err, tt.errorMsg)
				}
				return
			}
			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// mockClusterConfigClient is a mock implementation of ClusterConfigServiceClient
type mockClusterConfigClient struct {
	clusterconfigv1.ClusterConfigServiceClient
	getClusterAccessGraphConfigResp *clusterconfigv1.GetClusterAccessGraphConfigResponse
	getClusterAccessGraphConfigErr  error
}

func (m *mockClusterConfigClient) GetClusterAccessGraphConfig(ctx context.Context, req *clusterconfigv1.GetClusterAccessGraphConfigRequest, opts ...grpc.CallOption) (*clusterconfigv1.GetClusterAccessGraphConfigResponse, error) {
	if m.getClusterAccessGraphConfigErr != nil {
		return nil, m.getClusterAccessGraphConfigErr
	}
	return m.getClusterAccessGraphConfigResp, nil
}
