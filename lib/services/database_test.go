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
	"bufio"
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/stretchr/testify/require"
)

// TestDatabaseUnmarshal verifies a database resource can be unmarshaled.
func TestDatabaseUnmarshal(t *testing.T) {
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "test-database",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		CACert:   fixtures.TLSCACertPEM,
	})
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(fmt.Sprintf(databaseYAML, indent(fixtures.TLSCACertPEM, 4))))
	require.NoError(t, err)
	actual, err := UnmarshalDatabase(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestDatabaseMarshal verifies a marshaled database resource can be unmarshaled back.
func TestDatabaseMarshal(t *testing.T) {
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "test-database",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		CACert:   fixtures.TLSCACertPEM,
	})
	require.NoError(t, err)
	data, err := MarshalDatabase(expected)
	require.NoError(t, err)
	actual, err := UnmarshalDatabase(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// indent returns the string where each line is indented by the specified
// number of spaces.
func indent(s string, spaces int) string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, fmt.Sprintf("%v%v", strings.Repeat(" ", spaces), scanner.Text()))
	}
	return strings.Join(lines, "\n")
}

var databaseYAML = `kind: db
version: v3
metadata:
  name: test-database
  description: "Test description"
  labels:
    env: dev
spec:
  protocol: "postgres"
  uri: "localhost:5432"
  ca_cert: |
%v`
