/*
Copyright 2022 Gravitational, Inc.

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

package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
)

// MySQLPinger implements the DatabasePinger interface for the MySQL protocol.
type MySQLPinger struct{}

// convertError converts the error from MySQL client since it can be wrapped in an [errors.Causer].
// The MySQL engine in the agent already does this, but we need it here because
// the error is from the MySQL client.
func convertError(err error) error {
	// causer defines an interface for errors wrapped by the [errors] package.
	type causer interface {
		Cause() error
	}

	if causer, ok := err.(causer); ok {
		return trace.Wrap(causer.Cause())
	}

	return trace.Wrap(err)
}

// Ping connects to the database and issues a basic select statement to validate the connection.
func (p *MySQLPinger) Ping(ctx context.Context, params PingParams) error {
	if err := params.CheckAndSetDefaults(defaults.ProtocolMySQL); err != nil {
		return trace.Wrap(err)
	}

	var nd net.Dialer
	addr := fmt.Sprintf("%s:%d", params.Host, params.Port)
	conn, err := client.ConnectWithDialer(ctx, "tcp", addr,
		params.Username,
		"", // no password, we're dialing into a tunnel.
		params.DatabaseName,
		nd.DialContext,
	)
	if err != nil {
		return convertError(err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			logrus.WithError(err).Info("Failed to close connection in MySQLPinger.Ping")
		}
	}()

	if err := conn.Ping(); err != nil {
		return convertError(err)
	}

	return nil
}

// IsConnectionRefusedError checks whether the error is of type connection refused.
func (p *MySQLPinger) IsConnectionRefusedError(err error) bool {
	if err == nil {
		return false
	}

	var mErr *mysql.MyError
	if errors.As(err, &mErr) {
		switch mErr.Code {
		case mysql.ER_HOST_NOT_PRIVILEGED,
			mysql.ER_HOST_IS_BLOCKED:
			return true
		case mysql.ER_UNKNOWN_ERROR:
			// check error substrings if the error code is unknown.
		default:
			return false
		}
	}
	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "connection refused"):
		return true
	case strings.Contains(errMsg, "host"):
		return strings.Contains(errMsg, "is blocked") || strings.Contains(errMsg, "is not allowed to connect")
	}
	return false
}

// IsInvalidDatabaseUserError checks whether the error is of type invalid database user.
// This can happen when the user doesn't exist or the user was created with a cert
// subject CN that does not match the user name.
func (p *MySQLPinger) IsInvalidDatabaseUserError(err error) bool {
	if err == nil {
		return false
	}

	var mErr *mysql.MyError
	if errors.As(err, &mErr) {
		switch mErr.Code {
		case mysql.ER_ACCESS_DENIED_ERROR,
			mysql.ER_ACCESS_DENIED_NO_PASSWORD_ERROR,
			mysql.ER_USERNAME:
			return true
		case mysql.ER_UNKNOWN_ERROR:
			// check error substrings if the error code is unknown.
		default:
			return false
		}
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "access denied for user") && !strings.Contains(errMsg, "to database")
}

// IsInvalidDatabaseNameError checks whether the error is of type invalid database name.
// This can happen when the database doesn't exist or the user want not granted permission
// to access that database in MySQL.
func (p *MySQLPinger) IsInvalidDatabaseNameError(err error) bool {
	if err == nil {
		return false
	}

	var mErr *mysql.MyError
	if errors.As(err, &mErr) {
		switch mErr.Code {
		case mysql.ER_BAD_DB_ERROR,
			mysql.ER_DBACCESS_DENIED_ERROR:
			return true
		case mysql.ER_UNKNOWN_ERROR:
			// check error substrings if the error code is unknown.
		default:
			return false
		}
	}
	errMsg := strings.ToLower(err.Error())
	isDeniedDB := strings.Contains(errMsg, "access denied for user") &&
		strings.Contains(errMsg, "to database")
	return isDeniedDB || strings.Contains(strings.ToLower(err.Error()), "unknown database")
}
