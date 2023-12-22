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

func TestGetClusterURI(t *testing.T) {
	tests := []struct {
		in  uri.ResourceURI
		out uri.ResourceURI
	}{
		{
			uri.NewClusterURI("cluster.sh"),
			uri.NewClusterURI("cluster.sh"),
		},
		{
			uri.NewClusterURI("cluster.sh").AppendServer("server1"),
			uri.NewClusterURI("cluster.sh"),
		},
		{
			uri.NewClusterURI("cluster.sh").AppendLeafCluster("leaf.sh"),
			uri.NewClusterURI("cluster.sh").AppendLeafCluster("leaf.sh"),
		},
		{
			uri.NewClusterURI("cluster.sh").AppendLeafCluster("leaf.sh").AppendDB("postgres"),
			uri.NewClusterURI("cluster.sh").AppendLeafCluster("leaf.sh"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.in.String(), func(t *testing.T) {
			require.Equal(t, tt.out, tt.in.GetClusterURI())
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

func TestIsRoot(t *testing.T) {
	tests := []struct {
		name   string
		in     uri.ResourceURI
		expect require.BoolAssertionFunc
	}{
		{
			name:   "root cluster URI",
			in:     uri.NewClusterURI("foo"),
			expect: require.True,
		},
		{
			name:   "leaf cluster URI",
			in:     uri.NewClusterURI("foo").AppendLeafCluster("leaf"),
			expect: require.False,
		},
		{
			name:   "root cluster resource URI",
			in:     uri.NewClusterURI("foo").AppendServer("bar"),
			expect: require.True,
		},
		{
			name:   "leaf cluster resource URI",
			in:     uri.NewClusterURI("foo").AppendLeafCluster("leaf").AppendServer("bar"),
			expect: require.False,
		},
		{
			name:   "gateway URI",
			in:     uri.NewGatewayURI("gateway"),
			expect: require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expect(t, tt.in.IsRoot())
		})
	}
}

func TestIsLeaf(t *testing.T) {
	tests := []struct {
		name   string
		in     uri.ResourceURI
		expect require.BoolAssertionFunc
	}{
		{
			name:   "root cluster URI",
			in:     uri.NewClusterURI("foo"),
			expect: require.False,
		},
		{
			name:   "leaf cluster URI",
			in:     uri.NewClusterURI("foo").AppendLeafCluster("leaf"),
			expect: require.True,
		},
		{
			name:   "root cluster resource URI",
			in:     uri.NewClusterURI("foo").AppendServer("bar"),
			expect: require.False,
		},
		{
			name:   "leaf cluster resource URI",
			in:     uri.NewClusterURI("foo").AppendLeafCluster("leaf").AppendServer("bar"),
			expect: require.True,
		},
		{
			name:   "gateway URI",
			in:     uri.NewGatewayURI("gateway"),
			expect: require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expect(t, tt.in.IsLeaf())
		})
	}
}
