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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	azurelib "github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/defaults"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
)

// ConvertError converts errors to trace errors.
func ConvertError(err error) error {
	if err == nil {
		return nil
	}
	// Unwrap original error first.
	var traceErr *trace.TraceErr
	if errors.As(err, &traceErr) {
		return ConvertError(trace.Unwrap(err))
	}
	var pgErr pgError
	if errors.As(err, &pgErr) {
		return ConvertError(pgErr.Unwrap())
	}

	var c causer
	if errors.As(err, &c) {
		return ConvertError(c.Cause())
	}
	if _, ok := status.FromError(err); ok {
		return trail.FromGRPC(err)
	}

	var googleAPIErr *googleapi.Error
	var awsRequestFailureErr *awshttp.ResponseError
	var azResponseErr *azcore.ResponseError
	var pgError *pgconn.PgError
	var myError *mysql.MyError
	switch err := trace.Unwrap(err); {
	case errors.As(err, &googleAPIErr):
		return convertGCPError(googleAPIErr)
	case errors.As(err, &awsRequestFailureErr):
		return awslib.ConvertRequestFailureError(awsRequestFailureErr)
	case errors.As(err, &azResponseErr):
		return azurelib.ConvertResponseError(azResponseErr)
	case errors.As(err, &pgError):
		return convertPostgresError(pgError)
	case errors.As(err, &myError):
		return convertMySQLError(myError)
	}
	return err // Return unmodified.
}

// convertGCPError converts GCP errors to trace errors.
func convertGCPError(err *googleapi.Error) error {
	switch err.Code {
	case http.StatusForbidden:
		return trace.AccessDenied("%s", err)
	case http.StatusConflict:
		return trace.CompareFailed("%s", err)
	}
	return err // Return unmodified.
}

// convertPostgresError converts Postgres driver errors to trace errors.
func convertPostgresError(err *pgconn.PgError) error {
	switch err.Code {
	case pgerrcode.InvalidAuthorizationSpecification, pgerrcode.InvalidPassword:
		return trace.AccessDenied("%s", err)
	}
	return err // Return unmodified.
}

// convertMySQLError converts MySQL driver errors to trace errors.
func convertMySQLError(err *mysql.MyError) error {
	switch err.Code {
	case mysql.ER_ACCESS_DENIED_ERROR, mysql.ER_DBACCESS_DENIED_ERROR:
		return trace.AccessDenied("%s", fmtEscape(err))
	}
	return err // Return unmodified.
}

// fmtEscape escapes "%" in the original error message to prevent fmt from
// thinking some args are missing.
func fmtEscape(err error) string {
	return strings.ReplaceAll(err.Error(), "%", "%%")
}

// causer defines an interface for errors wrapped by the "errors" package.
type causer interface {
	Cause() error
}

// pgError defines an interface for errors wrapped by Postgres driver.
type pgError interface {
	Unwrap() error
}

// ConvertConnectError converts common connection errors to trace errors with
// extra information/recommendations if necessary.
func ConvertConnectError(err error, sessionCtx *Session) error {
	if err == nil {
		return nil
	}

	errString := err.Error()
	switch {
	case strings.Contains(errString, "x509: certificate signed by unknown authority"):
		return trace.AccessDenied("Database service cannot validate database's certificate: %v. Please verify if the correct CA bundle is used in the database config.", err)
	case strings.Contains(errString, "x509: certificate has expired or is not yet valid"):
		return trace.ConnectionProblem(
			err,
			"Connection Failure. Database service could not validate database’s certificate: certificate expired or is not yet valid. "+
				"More info at: https://goteleport.com/docs/enroll-resources/database-access/troubleshooting#certificate-expired-or-is-not-yet-valid",
		)
	case strings.Contains(errString, "tls: unknown certificate authority"):
		return trace.AccessDenied("Database cannot validate client certificate generated by database service: %v.", err)
	}

	orgErr := err
	err = ConvertError(orgErr)

	if trace.IsAccessDenied(err) {
		switch sessionCtx.Database.GetType() {
		case types.DatabaseTypeElastiCache:
			return createElastiCacheRedisAccessDeniedError(err, sessionCtx)
		case types.DatabaseTypeMemoryDB:
			return createMemoryDBAccessDeniedError(err, sessionCtx)
		case types.DatabaseTypeRDS:
			return createRDSAccessDeniedError(err, orgErr, sessionCtx)
		case types.DatabaseTypeRDSProxy:
			return createRDSProxyAccessDeniedError(err, sessionCtx)
		case types.DatabaseTypeAzure:
			return createAzureAccessDeniedError(err, sessionCtx)
		}
	}

	return trace.Wrap(err)
}

// createElastiCacheRedisAccessDeniedError creates an error with help message
// to setup IAM auth for ElastiCache Redis.
func createElastiCacheRedisAccessDeniedError(err error, sessionCtx *Session) error {
	policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(sessionCtx.Database)
	if getPolicyErr != nil {
		policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
	}

	switch sessionCtx.Database.GetProtocol() {
	case defaults.ProtocolRedis:
		return trace.AccessDenied(`Could not connect to database:

  %v

Make sure that IAM auth is enabled for ElastiCache user %q and Teleport database
agent's IAM policy has "elasticache:Connect" permissions (note that IAM changes may
take a few minutes to propagate):

%v
`, err, sessionCtx.DatabaseUser, policy)

	default:
		return trace.Wrap(err)
	}
}

// createMemoryDBAccessDeniedError creates an error with help message
// to setup IAM auth for MemoryDB Redis.
func createMemoryDBAccessDeniedError(err error, sessionCtx *Session) error {
	policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(sessionCtx.Database)
	if getPolicyErr != nil {
		policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
	}

	switch sessionCtx.Database.GetProtocol() {
	case defaults.ProtocolRedis:
		return trace.AccessDenied(`Could not connect to database:

  %v

Make sure that IAM auth is enabled for MemoryDB user %q and the user is in the
ACL associated with the MemoryDB cluster. Also Teleport database agent's IAM
policy must have "memorydb:Connect" permissions (note that IAM changes may take
a few minutes to propagate):

%v
`, err, sessionCtx.DatabaseUser, policy)

	default:
		return trace.Wrap(err)
	}
}

func isRDSMySQLIAMAuthError(err error) bool {
	var c causer
	if errors.As(err, &c) {
		return isRDSMySQLIAMAuthError(c.Cause())
	}
	var mysqlError *mysql.MyError
	if !errors.As(trace.Unwrap(err), &mysqlError) {
		return false
	}
	return mysqlError.Code == mysql.ER_ACCESS_DENIED_ERROR
}

// createRDSAccessDeniedError creates an error with help message to setup IAM
// auth for RDS.
func createRDSAccessDeniedError(err, orgErr error, sessionCtx *Session) error {
	policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(sessionCtx.Database)
	if getPolicyErr != nil {
		policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
	}

	switch sessionCtx.Database.GetProtocol() {
	case defaults.ProtocolMySQL:
		// Not all access denied errors are IAM Auth errors, so check again.
		if !isRDSMySQLIAMAuthError(orgErr) {
			return trace.Wrap(err)
		}

		return trace.AccessDenied(`Could not connect to database:

  %v

Make sure that IAM auth is enabled for MySQL user %q and Teleport database
agent's IAM policy has "rds-connect" permissions (note that IAM changes may
take a few minutes to propagate):

%v
`, err, sessionCtx.DatabaseUser, policy)

	case defaults.ProtocolPostgres:
		return trace.AccessDenied(`Could not connect to database:

  %v

Make sure that Postgres user %q has "rds_iam" role and Teleport database
agent's IAM policy has "rds-connect" permissions (note that IAM changes may
take a few minutes to propagate):

%v
`, err, sessionCtx.DatabaseUser, policy)

	default:
		return trace.Wrap(err)
	}
}

// createRDSProxyAccessDeniedError creates an error with help message to setup
// IAM auth for RDS Proxy.
func createRDSProxyAccessDeniedError(err error, sessionCtx *Session) error {
	policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(sessionCtx.Database)
	if getPolicyErr != nil {
		policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
	}

	return trace.AccessDenied(`Could not connect to database:

  %v

Make sure credentials for %v user %q is available in one of the Secrets Manager
secrets associated with the RDS Proxy and the IAM Authentication is set to
"required" for that secret.

Also, make sure the Teleport database agent's IAM policy has "rds-connect"
permissions (note that IAM changes may take a few minutes to propagate):

%v
`,
		err,
		defaults.ReadableDatabaseProtocol(sessionCtx.Database.GetProtocol()),
		sessionCtx.DatabaseUser,
		policy,
	)
}

// createAzureAccessDeniedError creates an error with help message to setup AAD
// auth for PostgreSQL/MySQL.
func createAzureAccessDeniedError(err error, sessionCtx *Session) error {
	switch sessionCtx.Database.GetProtocol() {
	case defaults.ProtocolMySQL:
		return trace.AccessDenied(`Could not connect to database:

  %v

Make sure that Azure Active Directory auth is configured for MySQL user %q and the Teleport database
agent's service principal. See: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql/
`, err, sessionCtx.DatabaseUser)
	case defaults.ProtocolPostgres:
		return trace.AccessDenied(`Could not connect to database:

  %v

Make sure that Azure Active Directory auth is configured for Postgres user %q and the Teleport database
agent's service principal. See: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql/
`, err, sessionCtx.DatabaseUser)
	default:
		return trace.Wrap(err)
	}
}
