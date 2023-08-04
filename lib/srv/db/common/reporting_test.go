// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestReporterMetadataFromProxyCtx(t *testing.T) {
	tests := []struct {
		name     string
		proxyCtx *ProxyContext
		want     string
	}{
		{
			name:     "no servers, metadata from identity",
			proxyCtx: &ProxyContext{Identity: tlsca.Identity{RouteToDatabase: tlsca.RouteToDatabase{Protocol: "foobar"}}},
			want:     "foobar;unknown",
		},
		{
			name: "servers, metadata from db info",
			proxyCtx: &ProxyContext{Servers: []types.DatabaseServer{&types.DatabaseServerV3{
				Spec: types.DatabaseServerSpecV3{
					Database: &types.DatabaseV3{
						Spec: types.DatabaseSpecV3{
							Protocol: types.DatabaseProtocolPostgreSQL,
							URI:      "postgres.example.com:5432",
						},
					},
				},
			}}},
			want: "postgres;self-hosted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ReporterMetadataFromProxyCtx(tt.proxyCtx))
		})
	}
}
