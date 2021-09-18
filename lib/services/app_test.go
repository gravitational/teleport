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

package services

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/stretchr/testify/require"
)

// TestApplicationUnmarshal verifies an app resource can be unmarshaled.
func TestApplicationUnmarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(appYAML))
	require.NoError(t, err)
	actual, err := UnmarshalApp(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestApplicationMarshal verifies a marshaled app resource can be unmarshaled back.
func TestApplicationMarshal(t *testing.T) {
	expected, err := types.NewAppV3(types.Metadata{
		Name:        "test-app",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	data, err := MarshalApp(expected)
	require.NoError(t, err)
	actual, err := UnmarshalApp(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var appYAML = `kind: app
version: v3
metadata:
  name: test-app
  description: "Test description"
  labels:
    env: dev
spec:
  uri: "http://localhost:8080"`
