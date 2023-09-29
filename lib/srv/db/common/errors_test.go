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
