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

package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func Test_maybeHashUsername(t *testing.T) {
	tests := []struct {
		input      string
		wantOutput string
	}{
		{
			input:      "short-name",
			wantOutput: "short-name",
		},
		{
			input:      "a-very-very-very-long-name-that-is-over-32",
			wantOutput: "teleport-093344e2a9988fd4",
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			output := maybeHashUsername(test.input, mysqlMaxUsernameLength)
			require.Equal(t, test.wantOutput, output)
			require.Less(t, len(output), mysqlMaxUsernameLength)
		})
	}
}

func Test_makeActivateUserDetails(t *testing.T) {
	rds, err := types.NewDatabaseV3(types.Metadata{
		Name: "RDS",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
	})
	require.NoError(t, err)

	teleportUsername := "a-very-very-very-long-name-that-is-over-32"
	details, err := makeActivateUserDetails(
		&common.Session{
			Database:      rds,
			DatabaseUser:  maybeHashUsername(teleportUsername, mysqlMaxUsernameLength),
			DatabaseRoles: []string{"role", "role2"},
		},
		teleportUsername,
	)
	require.NoError(t, err)

	wantOutput := `{"roles":["role","role2"],"auth_options":"IDENTIFIED WITH AWSAuthenticationPlugin AS \\"RDS\\"","attributes":{"user":"a-very-very-very-long-name-that-is-over-32"}}`
	require.Equal(t, wantOutput, details)
}
