/*
Copyright 2023 Gravitational, Inc.

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
