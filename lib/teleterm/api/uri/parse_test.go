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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in         string
		out        uri.ResourceURI
		checkError require.ErrorAssertionFunc
	}{
		{
			in:         uri.NewClusterURI("foo").String(),
			out:        uri.NewClusterURI("foo"),
			checkError: require.NoError,
		},
		{
			in:         uri.NewClusterURI("foo").AppendLeafCluster("bar").String(),
			out:        uri.NewClusterURI("foo").AppendLeafCluster("bar"),
			checkError: require.NoError,
		},
		{
			in:         uri.NewClusterURI("foo").AppendDB("db").String(),
			out:        uri.NewClusterURI("foo").AppendDB("db"),
			checkError: require.NoError,
		},
		{
			in:         uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("kube").String(),
			out:        uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("kube"),
			checkError: require.NoError,
		},
		{
			in:         uri.NewGatewayURI("foo").String(),
			checkError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			out, err := uri.Parse(tt.in)
			tt.checkError(t, err)
			require.Equal(t, tt.out, out)
		})
	}
}

func TestParseGatewayTargetURI(t *testing.T) {
	tests := []struct {
		in         string
		out        uri.ResourceURI
		checkError require.ErrorAssertionFunc
	}{
		{
			in:         uri.NewClusterURI("foo").String(),
			checkError: require.Error,
		},
		{
			in:         uri.NewClusterURI("foo").AppendLeafCluster("bar").String(),
			checkError: require.Error,
		},
		{
			in:         uri.NewClusterURI("foo").AppendDB("db").String(),
			out:        uri.NewClusterURI("foo").AppendDB("db"),
			checkError: require.NoError,
		},
		{
			in:         uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("kube").String(),
			out:        uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendKube("kube"),
			checkError: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			out, err := uri.ParseGatewayTargetURI(tt.in)
			tt.checkError(t, err)
			require.Equal(t, tt.out, out)
		})
	}
}

func TestParseDBURI(t *testing.T) {
	tests := []struct {
		in         string
		out        uri.ResourceURI
		checkError require.ErrorAssertionFunc
	}{
		{
			in:         uri.NewClusterURI("foo").AppendKube("kube").String(),
			checkError: require.Error,
		},
		{
			in:         uri.NewClusterURI("foo").AppendLeafCluster("bar").String(),
			checkError: require.Error,
		},
		{
			in:         uri.NewClusterURI("foo").AppendDB("db").String(),
			out:        uri.NewClusterURI("foo").AppendDB("db"),
			checkError: require.NoError,
		},
		{
			in:         uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendDB("db").String(),
			out:        uri.NewClusterURI("foo").AppendLeafCluster("bar").AppendDB("db"),
			checkError: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			out, err := uri.ParseDBURI(tt.in)
			tt.checkError(t, err)
			require.Equal(t, tt.out, out)
		})
	}
}
