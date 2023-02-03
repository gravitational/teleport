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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDiscoveredServerUnmarshal verifies that DiscoveredServer resource can be unmarshaled.
func TestDiscoveredServerUnmarshal(t *testing.T) {
	expected, err := types.NewDiscoveredServerV1(types.Metadata{
		Name: "test-discovered-server",
	}, types.DiscoveredServerSpecV1{
		Labels: map[string]string{
			"key_test": "value_test",
		}})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(discoveredServerYAML))
	require.NoError(t, err)
	actual, err := UnmarshalDiscoveredServer(data)
	require.NoError(t, err)

	require.Equal(t, expected, actual)
}

// TestDiscoveredServerMarshal verifies a marshaled discovered server resource resource can be unmarshaled back.
func TestDiscoveredServerMarshal(t *testing.T) {
	expected, err := types.NewDiscoveredServerV1(types.Metadata{
		Name: "test-discovered-server",
	}, types.DiscoveredServerSpecV1{
		Labels: map[string]string{
			"key_test": "value_test",
		}})
	require.NoError(t, err)
	data, err := MarshalDiscoveredServer(expected)
	require.NoError(t, err)
	actual, err := UnmarshalDiscoveredServer(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var discoveredServerYAML = `---
kind: discovered_server
version: v1
metadata:
  name: test-discovered-server
spec:
  labels:
    key_test: value_test
`
