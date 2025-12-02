/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

const (
	// gcpMySQLDBUserTypeBuiltIn indicates the database's built-in user type.
	gcpMySQLDBUserTypeBuiltIn = "BUILT_IN"
	// gcpMySQLDBUserTypeServiceAccount indicates a Cloud IAM service account.
	gcpMySQLDBUserTypeServiceAccount = "CLOUD_IAM_SERVICE_ACCOUNT"
	//  gcpMySQLDBUserTypeGroupServiceAccount indicates a Cloud IAM group service account.
	gcpMySQLDBUserTypeGroupServiceAccount = "CLOUD_IAM_GROUP_SERVICE_ACCOUNT"
	// gcpMySQLDBUserTypeUser indicates a Cloud IAM user.
	gcpMySQLDBUserTypeUser = "CLOUD_IAM_USER"
	// gcpMySQLDBUserTypeGroupUser indicates a Cloud IAM group login user.
	gcpMySQLDBUserTypeGroupUser = "CLOUD_IAM_GROUP_USER"
)

const (
	// gcpSQLListenPort is the port used by Cloud SQL MySQL instances.
	gcpSQLListenPort = "3306"
	// gcpSQLProxyListenPort is the port used by Cloud Proxy for MySQL instances.
	gcpSQLProxyListenPort = "3307"
)

func isDBUserFullGCPServerAccountID(dbUser string) bool {
	// Example: mysql-iam-user@my-project-id.iam.gserviceaccount.com
	return strings.Contains(dbUser, "@") &&
		strings.HasSuffix(dbUser, ".iam.gserviceaccount.com")
}

func isDBUserShortGCPServiceAccountID(dbUser string) bool {
	// Example: mysql-iam-user@my-project-id.iam
	return strings.Contains(dbUser, "@") &&
		strings.HasSuffix(dbUser, ".iam")
}

func gcpServiceAccountToDatabaseUser(serviceAccountName string) string {
	user, _, _ := strings.Cut(serviceAccountName, "@")
	return user
}

func databaseUserToGCPServiceAccount(database types.Database, dbUser string) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com", dbUser, database.GetGCP().ProjectID)
}

type gcpClients interface {
	GetGCPSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
}

type gcpAuth struct {
	auth         common.Auth
	authClient   *authclient.Client
	clients      gcpClients
	clock        clockwork.Clock
	database     types.Database
	databaseUser string
	log          *slog.Logger

	requireSSL    bool
	requireSSLErr error
	sometimes     *rate.Sometimes
}

func (a *gcpAuth) getGCPUserAndPassword(ctx context.Context) (string, string, error) {
	gcpClient, err := a.clients.GetGCPSQLAdminClient(ctx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// If `--db-user` is the full service account email ID, use IAM Auth.
	if isDBUserFullGCPServerAccountID(a.databaseUser) {
		user := gcpServiceAccountToDatabaseUser(a.databaseUser)
		password, err := a.getGCPIAMAuthToken(ctx, a.databaseUser)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		return user, password, nil
	}

	// Note that GCP Postgres' format "user@my-project-id.iam" is not accepted
	// for GCP MySQL. For GCP Postgres, "user@my-project-id.iam" is the actual
	// mapped in-database username. However, the mapped in-database username
	// for GCP MySQL does not have the "@my-project-id.iam" part.
	if isDBUserShortGCPServiceAccountID(a.databaseUser) {
		return "", "", trace.BadParameter("username %q is not accepted for GCP MySQL. Please use the in-database username or the full service account Email ID.", a.databaseUser)
	}

	// Get user info to decide how to authenticate.
	user := a.databaseUser
	dbUserInfo, err := gcpClient.GetUser(ctx, a.database, a.databaseUser)
	switch {
	// GetUser permission is new for IAM auth. If no permission, assume legacy password user.
	case trace.IsAccessDenied(err):
		a.log.DebugContext(ctx, "Access denied to get GCP MySQL database user info, continuing with password auth",
			"user", a.databaseUser,
		)
		password, err := a.getGCPOneTimePassword(ctx)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		return user, password, nil

	// Make the original error message "object not found" more readable. Note
	// that catching not found here also prevents Google creating a new
	// database user during OTP generation.
	case trace.IsNotFound(err):
		return "", "", trace.NotFound("database user %q does not exist in database %q", a.databaseUser, a.database.GetName())

	// Report any other error.
	case err != nil:
		return "", "", trace.Wrap(err)
	}

	// The user type constants are documented in their SDK. However, in
	// practice, type can also be empty for built-in user.
	switch dbUserInfo.Type {
	case "",
		gcpMySQLDBUserTypeBuiltIn:
		password, err := a.getGCPOneTimePassword(ctx)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		return user, password, nil

	case gcpMySQLDBUserTypeServiceAccount,
		gcpMySQLDBUserTypeGroupServiceAccount:
		serviceAccountName := databaseUserToGCPServiceAccount(a.database, a.databaseUser)
		password, err := a.getGCPIAMAuthToken(ctx, serviceAccountName)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		return user, password, nil

	case gcpMySQLDBUserTypeUser,
		gcpMySQLDBUserTypeGroupUser:
		return "", "", trace.BadParameter("GCP MySQL user type %q not supported", dbUserInfo.Type)

	default:
		return "", "", trace.BadParameter("unknown GCP MySQL user type %q", dbUserInfo.Type)
	}
}

func (a *gcpAuth) getGCPIAMAuthToken(ctx context.Context, dbUser string) (string, error) {
	a.log.DebugContext(ctx, "Authenticating GCP MySQL with IAM auth", "db_user", dbUser)

	// Note that sessionCtx.DatabaseUser is the service account.
	password, err := a.auth.GetCloudSQLAuthToken(ctx, dbUser)
	return password, trace.Wrap(err)
}

func (a *gcpAuth) getGCPOneTimePassword(ctx context.Context) (string, error) {
	a.log.DebugContext(ctx, "Authenticating GCP MySQL with password auth")

	// For Cloud SQL MySQL legacy auth, we use one-time passwords by resetting
	// the database user password for each connection. Thus, acquire a lock to
	// make sure all connection attempts to the same database and user are
	// serialized.
	retryCtx, cancel := context.WithTimeout(ctx, defaults.DatabaseConnectTimeout)
	defer cancel()
	lease, err := services.AcquireSemaphoreWithRetry(retryCtx, a.makeAcquireSemaphoreConfig())
	if err != nil {
		return "", trace.Wrap(err)
	}
	// Only release the semaphore after the connection has been established
	// below. If the semaphore fails to release for some reason, it will
	// expire in a minute on its own.
	defer func() {
		err := a.authClient.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			a.log.ErrorContext(ctx, "Failed to cancel lease", "lease", lease, "error", err)
		}
	}()
	password, err := a.auth.GetCloudSQLPassword(ctx, a.database, a.databaseUser)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return password, nil
}

// makeAcquireSemaphoreConfig builds parameters for acquiring a semaphore
// for connecting to a MySQL Cloud SQL instance for this session.
func (a *gcpAuth) makeAcquireSemaphoreConfig() services.AcquireSemaphoreWithRetryConfig {
	return services.AcquireSemaphoreWithRetryConfig{
		Service: a.authClient,
		// The semaphore will serialize connections to the database as specific
		// user. If we fail to release the lock for some reason, it will expire
		// in a minute anyway.
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: "gcp-mysql-token",
			SemaphoreName: fmt.Sprintf("%v-%v", a.database.GetName(), a.databaseUser),
			MaxLeases:     1,
		},
		// If multiple connections are being established simultaneously to the
		// same database as the same user, retry for a few seconds.
		Retry: retryutils.LinearConfig{
			Step:  time.Second,
			Max:   time.Second,
			Clock: a.clock,
		},
		TTL: time.Minute,
		Now: a.clock.Now,
	}
}

func (a *gcpAuth) checkSSLRequired(ctx context.Context) (bool, error) {
	a.sometimes.Do(func() {
		client, err := a.clients.GetGCPSQLAdminClient(ctx)
		if err != nil {
			a.requireSSL = false
			a.requireSSLErr = err
			return
		}
		a.requireSSL, a.requireSSLErr = checkGCPRequireSSL(ctx, a.database, client)
	})
	return a.requireSSL, trace.Wrap(a.requireSSLErr)
}

func checkGCPRequireSSL(ctx context.Context, database types.Database, client gcp.SQLAdminClient) (bool, error) {
	// Detect whether the instance is set to require SSL.
	// Fallback to not requiring SSL for access denied errors.
	requireSSL, err := cloud.GetGCPRequireSSL(ctx, database, client)
	if err != nil {
		if trace.IsAccessDenied(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return requireSSL, nil
}

func (a *gcpAuth) appendGCPClientCert(ctx context.Context, certExpiry time.Time, tlsConfig *tls.Config) error {
	gcpClient, err := a.clients.GetGCPSQLAdminClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(cloud.AppendGCPClientCert(ctx, &cloud.AppendGCPClientCertRequest{
		GCPClient:   gcpClient,
		GenerateKey: a.auth.GenerateDatabaseClientKey,
		Expiry:      certExpiry,
		Database:    a.database,
		TLSConfig:   tlsConfig,
	}))
}

// newGCPTLSDialer returns a TLS dialer configured to connect to the Cloud Proxy
// port rather than the default MySQL port.
func newGCPTLSDialer(tlsConfig *tls.Config) client.Dialer {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		// Workaround issue generating ephemeral certificates for secure connections
		// by creating a TLS connection to the Cloud Proxy port overriding the
		// MySQL client's connection. MySQL on the default port does not trust
		// the ephemeral certificate's CA but Cloud Proxy does.
		address = getGCPTLSAddress(address)
		tlsDialer := tls.Dialer{Config: tlsConfig}
		return tlsDialer.DialContext(ctx, network, address)
	}
}

// getGCPTLSAddress returns the appropriate address for a Cloud SQL MySQL
// instance, possibly overriding the default port to instead use the Cloud Proxy
// port.
func getGCPTLSAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err == nil && port == gcpSQLListenPort {
		return net.JoinHostPort(host, gcpSQLProxyListenPort)
	}
	return address
}
