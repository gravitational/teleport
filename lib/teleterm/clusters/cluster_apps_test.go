// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package clusters

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func TestAssembleAppFQDN(t *testing.T) {
	rootCluster := Cluster{
		ProfileName: "example.com",
		status: client.ProfileStatus{
			ProxyURL: url.URL{
				Host: "example.com:3080",
			},
		},
		URI: uri.NewClusterURI("example.com"),
	}

	leafCluster := Cluster{
		ProfileName: "example.com",
		status: client.ProfileStatus{
			ProxyURL: url.URL{
				Host: "example.com:3080",
			},
		},
		URI: uri.NewClusterURI("example.com").AppendLeafCluster("bar"),
	}

	app, err := types.NewAppV3(types.Metadata{Name: "dumper"}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)

	appWithPublicAddr, err := types.NewAppV3(types.Metadata{Name: "dumper"}, types.AppSpecV3{
		URI:        "http://localhost:8080",
		PublicAddr: "dumper.net",
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		cluster Cluster
		app     types.Application
		want    string
	}{
		{
			name:    "root cluster app",
			cluster: rootCluster,
			app:     app,
			want:    "dumper.example.com",
		},
		{
			name:    "leaf cluster app",
			cluster: leafCluster,
			app:     app,
			want:    "dumper.example.com",
		},
		{
			name:    "root cluster app with public address",
			cluster: rootCluster,
			app:     appWithPublicAddr,
			want:    "dumper.net",
		},
		{
			name:    "leaf cluster app with public address",
			cluster: leafCluster,
			app:     appWithPublicAddr,
			want:    "dumper.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fqdn := tt.cluster.AssembleAppFQDN(tt.app)
			require.Equal(t, tt.want, fqdn)
		})
	}
}
