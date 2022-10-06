/*
Copyright 2021 Gravitational, Inc.

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

package uri_test

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  uri.ResourceURI
		out string
	}{
		{
			uri.NewClusterURI("teleport.sh").AppendServer("server1"),
			"/clusters/teleport.sh/servers/server1",
		},
		{
			uri.NewClusterURI("teleport.sh").AppendApp("app1"),
			"/clusters/teleport.sh/apps/app1",
		},
		{
			uri.NewClusterURI("teleport.sh").AppendDB("dbhost1"),
			"/clusters/teleport.sh/dbs/dbhost1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%v", tt.in), func(t *testing.T) {
			t.Parallel()

			out := tt.in.String()
			require.Equal(t, tt.out, out)
		})
	}
}

func TestParseClusterURI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  string
		out uri.ResourceURI
	}{
		{
			"/clusters/cluster.sh",
			uri.NewClusterURI("cluster.sh"),
		},
		{
			"/clusters/cluster.sh/servers/server1",
			uri.NewClusterURI("cluster.sh"),
		},
		{
			"/clusters/cluster.sh/leaves/leaf.sh",
			uri.NewClusterURI("cluster.sh").AppendLeafCluster("leaf.sh"),
		},
		{
			"/clusters/cluster.sh/leaves/leaf.sh/dbs/postgres",
			uri.NewClusterURI("cluster.sh").AppendLeafCluster("leaf.sh"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			out, err := uri.ParseClusterURI(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.out, out)
		})
	}
}
