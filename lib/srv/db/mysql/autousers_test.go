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

package mysql

import (
	"context"
	"errors"
	"log/slog"
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
		State:   common.SQLStateUsernameDoesNotMatch,
		Message: `Teleport username does not match user attributes`,
	}
	rolesChangedError := &mysql.MyError{
		Code:    mysql.ER_SIGNAL_EXCEPTION,
		State:   common.SQLStateRolesChanged,
		Message: `user has active connections and roles have changed`,
	}
	// Currently not converted to trace.AccessDeined as it may conflict with
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
				return errors.Is(trace.Unwrap(err), permissionError)
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

func Test_checkMySQLSupportedVersion(t *testing.T) {
	tests := []struct {
		input      string
		checkError require.ErrorAssertionFunc
	}{
		{
			input:      "invalid-server-version",
			checkError: require.NoError,
		},
		{
			input:      "8.0.28",
			checkError: require.NoError,
		},
		{
			input:      "9.0.0",
			checkError: require.NoError,
		},
		{
			input:      "5.7.42",
			checkError: require.Error,
		},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			test.checkError(t, checkMySQLSupportedVersion(context.Background(), slog.Default(), test.input))
		})
	}
}

func Test_checkMariaDBSupportedVersion(t *testing.T) {
	tests := []struct {
		input      string
		checkError require.ErrorAssertionFunc
	}{
		{
			input:      "invalid-server-version",
			checkError: require.NoError,
		},
		{
			input:      "5.5.5-10.7.8-MariaDB-1:10.7.8+maria~ubu2004",
			checkError: require.NoError,
		},
		{
			input:      "5.5.5-10.9.8-MariaDB",
			checkError: require.NoError,
		},
		{
			input:      "5.5.5-10.3.3-MariaDB",
			checkError: require.NoError,
		},
		{
			input:      "5.5.5-10.2.11-MariaDB",
			checkError: require.NoError,
		},
		{
			input:      "11.0.3-MariaDB-1:11.0.3+maria~ubu2204",
			checkError: require.NoError,
		},
		{
			input:      "5.5.5-10.3.2-MariaDB",
			checkError: require.Error,
		},
		{
			input:      "5.5.5-10.2.10-MariaDB",
			checkError: require.Error,
		},
		{
			input:      "5.5.5-10.1.0-MariaDB",
			checkError: require.Error,
		},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			test.checkError(t, checkMariaDBSupportedVersion(context.Background(), slog.Default(), test.input))
		})
	}
}
