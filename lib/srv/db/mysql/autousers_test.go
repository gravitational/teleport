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

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
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
			wantOutput: "tp-XnfKd0MysfJ/xaR/b3OgoQvoTuo",
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

	wantOutput := `{"roles":["role","role2"],"auth_options":"IDENTIFIED WITH AWSAuthenticationPlugin AS \"RDS\"","attributes":{"user":"a-very-very-very-long-name-that-is-over-32"}}`
	require.Equal(t, wantOutput, string(details))
}

func Test_convertActivateError(t *testing.T) {
	sessionCtx := &common.Session{
		DatabaseUser: "user1",
		Identity: tlsca.Identity{
			Username: "user1",
		},
	}

	createUserFailedError := &mysql.MyError{
		Code:    mysql.ER_CANNOT_USER,
		State:   "HY000",
		Message: `Operation CREATE USER failed for 'user1'@'%'`,
	}
	usernameDoesNotMatchError := &mysql.MyError{
		Code:    mysql.ER_SIGNAL_EXCEPTION,
		State:   sqlStateUsernameDoesNotMatch,
		Message: `Teleport username does not match user attributes`,
	}
	rolesChangedError := &mysql.MyError{
		Code:    mysql.ER_SIGNAL_EXCEPTION,
		State:   sqlStateRolesChanged,
		Message: `user has active connections and roles have changed`,
	}
	// Current not converted to trace.AccessDeined as it may conflict with
	// common.ConvertConnectError.
	permissionError := &mysql.MyError{
		Code:    mysql.ER_SPECIFIC_ACCESS_DENIED_ERROR,
		State:   "42000",
		Message: `Access denied; you need (at least one of) the CREATE USER privilege(s) for this operation`,
	}

	tests := []struct {
		name          string
		input         error
		errorIs       func(error) bool
		errorContains string
	}{
		{
			name:          "create user failed",
			input:         createUserFailedError,
			errorIs:       trace.IsAlreadyExists,
			errorContains: "is not managed by Teleport",
		},
		{
			name:          "username does not match",
			input:         usernameDoesNotMatchError,
			errorIs:       trace.IsAlreadyExists,
			errorContains: "used for another Teleport user",
		},
		{
			name:          "roles changed",
			input:         trace.Wrap(rolesChangedError),
			errorIs:       trace.IsCompareFailed,
			errorContains: "quit all active connections",
		},
		{
			name:  "no permission",
			input: trace.Wrap(permissionError),
			errorIs: func(err error) bool {
				// Not converted.
				return trace.Unwrap(err) == permissionError
			},
			errorContains: permissionError.Message,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converted := convertActivateError(sessionCtx, test.input)
			require.True(t, test.errorIs(converted))
			require.Contains(t, converted.Error(), test.errorContains)
		})
	}
}
