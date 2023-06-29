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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func TestString(t *testing.T) {
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
		t.Run(fmt.Sprintf("%v", tt.in), func(t *testing.T) {
			out := tt.in.String()
			require.Equal(t, tt.out, out)
		})
	}
}

func TestParseClusterURI(t *testing.T) {
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
		t.Run(tt.in, func(t *testing.T) {
			out, err := uri.ParseClusterURI(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.out, out)
		})
	}
}

func TestGetDbName(t *testing.T) {
	tests := []struct {
		name string
		in   uri.ResourceURI
		out  string
	}{
		{
			name: "returns root cluster db name",
			in:   uri.NewClusterURI("foo").AppendDB("postgres"),
			out:  "postgres",
		},
		{
			name: "returns leaf cluster db name",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendDB("postgres"),
			out:  "postgres",
		},
		{
			name: "returns empty string when given root cluster URI",
			in:   uri.NewClusterURI("foo"),
			out:  "",
		},
		{
			name: "returns empty string when given leaf cluster URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			out:  "",
		},
		{
			name: "returns empty string when given root cluster non-db resource URI",
			in:   uri.NewClusterURI("foo").AppendKube("k8s"),
			out:  "",
		},
		{
			name: "returns empty string when given leaf cluster non-db resource URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("k8s"),
			out:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.in.GetDbName()
			require.Equal(t, tt.out, out)
		})
	}
}

func TestGetKubeName(t *testing.T) {
	tests := []struct {
		name string
		in   uri.ResourceURI
		out  string
	}{
		{
			name: "returns root cluster kube name",
			in:   uri.NewClusterURI("foo").AppendKube("k8s"),
			out:  "k8s",
		},
		{
			name: "returns leaf cluster kube name",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("k8s"),
			out:  "k8s",
		},
		{
			name: "returns empty string when given root cluster URI",
			in:   uri.NewClusterURI("foo"),
			out:  "",
		},
		{
			name: "returns empty string when given leaf cluster URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			out:  "",
		},
		{
			name: "returns empty string when given root cluster non-kube resource URI",
			in:   uri.NewClusterURI("foo").AppendDB("postgres"),
			out:  "",
		},
		{
			name: "returns empty string when given leaf cluster non-kube resource URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendDB("postgres"),
			out:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.in.GetKubeName()
			require.Equal(t, tt.out, out)
		})
	}
}

func TestGetServerUUID(t *testing.T) {
	tests := []struct {
		name string
		in   uri.ResourceURI
		out  string
	}{
		{
			name: "returns root cluster server UUID",
			in:   uri.NewClusterURI("foo").AppendServer("uuid"),
			out:  "uuid",
		},
		{
			name: "returns leaf cluster server UUID",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendServer("uuid"),
			out:  "uuid",
		},
		{
			name: "returns empty string when given root cluster URI",
			in:   uri.NewClusterURI("foo"),
			out:  "",
		},
		{
			name: "returns empty string when given leaf cluster URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			out:  "",
		},
		{
			name: "returns empty string when given root cluster non-server resource URI",
			in:   uri.NewClusterURI("foo").AppendKube("k8s"),
			out:  "",
		},
		{
			name: "returns empty string when given leaf cluster non-server resource URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("k8s"),
			out:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.in.GetServerUUID()
			require.Equal(t, tt.out, out)
		})
	}
}

func TestGetRootClusterURI(t *testing.T) {
	tests := []struct {
		name string
		in   uri.ResourceURI
		out  uri.ResourceURI
	}{
		{
			name: "noop on root cluster URI",
			in:   uri.NewClusterURI("foo"),
			out:  uri.NewClusterURI("foo"),
		},
		{
			name: "trims root cluster resource URI",
			in:   uri.NewClusterURI("foo").AppendDB("postgres"),
			out:  uri.NewClusterURI("foo"),
		},
		{
			name: "trims leaf cluster URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			out:  uri.NewClusterURI("foo"),
		},
		{
			name: "trims leaf cluster resource URI",
			in:   uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendDB("postgres"),
			out:  uri.NewClusterURI("foo"),
		},
		{
			name: "returns empty URI if given a gateway URI",
			in:   uri.NewGatewayURI("quux"),
			out:  uri.NewClusterURI(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.in.GetRootClusterURI()
			require.Equal(t, tt.out, out)
		})
	}
}

func TestIsDB(t *testing.T) {
	tests := []struct {
		in    uri.ResourceURI
		check require.BoolAssertionFunc
	}{
		{
			in:    uri.NewClusterURI("foo").AppendDB("db"),
			check: require.True,
		},
		{
			in:    uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendDB("db"),
			check: require.True,
		},
		{
			in:    uri.NewClusterURI("foo"),
			check: require.False,
		},
		{
			in:    uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			check: require.False,
		},
		{
			in:    uri.NewClusterURI("foo").AppendKube("kube"),
			check: require.False,
		},
	}

	for _, tt := range tests {
		tt.check(t, tt.in.IsDB())
	}
}

func TestIsKube(t *testing.T) {
	tests := []struct {
		in    uri.ResourceURI
		check require.BoolAssertionFunc
	}{
		{
			in:    uri.NewClusterURI("foo").AppendKube("kube"),
			check: require.True,
		},
		{
			in:    uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("kube"),
			check: require.True,
		},
		{
			in:    uri.NewClusterURI("foo"),
			check: require.False,
		},
		{
			in:    uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			check: require.False,
		},
		{
			in:    uri.NewClusterURI("foo").AppendDB("db"),
			check: require.False,
		},
	}

	for _, tt := range tests {
		tt.check(t, tt.in.IsKube())
	}
}
