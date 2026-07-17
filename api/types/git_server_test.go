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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitServerSSHEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		metadata *GitHubServerMetadata
		want     bool
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			want:     false,
		},
		{
			name:     "empty AllowProtocols defaults to SSH",
			metadata: &GitHubServerMetadata{},
			want:     true,
		},
		{
			name:     "explicit ssh",
			metadata: &GitHubServerMetadata{AllowProtocols: []string{GitProtocolSSH}},
			want:     true,
		},
		{
			name:     "explicit http only",
			metadata: &GitHubServerMetadata{AllowProtocols: []string{GitProtocolHTTP}},
			want:     false,
		},
		{
			name:     "both protocols",
			metadata: &GitHubServerMetadata{AllowProtocols: []string{GitProtocolSSH, GitProtocolHTTP}},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GitServerSSHEnabled(tt.metadata))
		})
	}
}

func TestGitServerHTTPEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		metadata *GitHubServerMetadata
		want     bool
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			want:     false,
		},
		{
			name:     "empty AllowProtocols defaults to no HTTP",
			metadata: &GitHubServerMetadata{},
			want:     false,
		},
		{
			name:     "explicit ssh only",
			metadata: &GitHubServerMetadata{AllowProtocols: []string{GitProtocolSSH}},
			want:     false,
		},
		{
			name:     "explicit http",
			metadata: &GitHubServerMetadata{AllowProtocols: []string{GitProtocolHTTP}},
			want:     true,
		},
		{
			name:     "both protocols",
			metadata: &GitHubServerMetadata{AllowProtocols: []string{GitProtocolSSH, GitProtocolHTTP}},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GitServerHTTPEnabled(tt.metadata))
		})
	}
}

func TestGitHubIntegrationSpecSSHEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec *GitHubIntegrationSpecV1
		want bool
	}{
		{
			name: "nil spec",
			spec: nil,
			want: false,
		},
		{
			name: "empty AllowProtocols defaults to SSH",
			spec: &GitHubIntegrationSpecV1{},
			want: true,
		},
		{
			name: "explicit ssh",
			spec: &GitHubIntegrationSpecV1{AllowProtocols: []string{GitProtocolSSH}},
			want: true,
		},
		{
			name: "explicit http only",
			spec: &GitHubIntegrationSpecV1{AllowProtocols: []string{GitProtocolHTTP}},
			want: false,
		},
		{
			name: "both protocols",
			spec: &GitHubIntegrationSpecV1{AllowProtocols: []string{GitProtocolSSH, GitProtocolHTTP}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.spec.SSHEnabled())
		})
	}
}

func TestGitHubIntegrationSpecHTTPEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec *GitHubIntegrationSpecV1
		want bool
	}{
		{
			name: "nil spec",
			spec: nil,
			want: false,
		},
		{
			name: "empty AllowProtocols defaults to no HTTP",
			spec: &GitHubIntegrationSpecV1{},
			want: false,
		},
		{
			name: "explicit ssh only",
			spec: &GitHubIntegrationSpecV1{AllowProtocols: []string{GitProtocolSSH}},
			want: false,
		},
		{
			name: "explicit http",
			spec: &GitHubIntegrationSpecV1{AllowProtocols: []string{GitProtocolHTTP}},
			want: true,
		},
		{
			name: "both protocols",
			spec: &GitHubIntegrationSpecV1{AllowProtocols: []string{GitProtocolSSH, GitProtocolHTTP}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.spec.HTTPEnabled())
		})
	}
}
