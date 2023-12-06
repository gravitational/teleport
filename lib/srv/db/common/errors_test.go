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

package common

import (
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func Test_isRDSMySQLIAMAuthError(t *testing.T) {
	iamAuthError := &mysql.MyError{
		Code:    mysql.ER_ACCESS_DENIED_ERROR,
		Message: "Access denied for user 'alice'@'10.0.0.197' (using password: YES)",
		State:   "28000",
	}
	require.True(t, isRDSMySQLIAMAuthError(iamAuthError))

	dbAccessError := &mysql.MyError{
		Code:    mysql.ER_DBACCESS_DENIED_ERROR,
		Message: "Access denied for user 'alice'@'%' to database 'db-no-access'",
		State:   "42000",
	}
	noDBError := &mysql.MyError{
		Code:    mysql.ER_BAD_DB_ERROR,
		Message: "Unknown database 'db-not-exist'",
		State:   "42000",
	}
	require.False(t, isRDSMySQLIAMAuthError(dbAccessError))
	require.False(t, isRDSMySQLIAMAuthError(noDBError))
	require.False(t, isRDSMySQLIAMAuthError(trace.AccessDenied("access denied")))
}
