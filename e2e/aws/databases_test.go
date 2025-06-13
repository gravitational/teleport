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

package e2e

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestDatabases(t *testing.T) {
	t.Parallel()
	testEnabled := os.Getenv(teleport.AWSRunDBTests)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		t.Skip("Skipping AWS Databases test suite.")
	}
	// when adding a new type of AWS db e2e test, you should add to this
	// unmatched discovery test and add a test for matched discovery/connection
	// as well below.
	t.Run("unmatched discovery", awsDBDiscoveryUnmatched)
	t.Run("rds", testRDS)
	t.Run("redshift serverless", testRedshiftServerless)
	t.Run("redshift cluster", testRedshiftCluster)
}

func awsDBDiscoveryUnmatched(t *testing.T) {
	t.Parallel()
	// get test settings
	awsRegion := mustGetEnv(t, awsRegionEnv)

	// setup discovery matchers
	var matchers []types.AWSMatcher
	for matcherType, assumeRoleARN := range map[string]string{
		// add a new matcher/role here to test that discovery properly
		// does *not* that kind of database for some unmatched tag.
		types.AWSMatcherRDS:                mustGetEnv(t, rdsDiscoveryRoleARNEnv),
		types.AWSMatcherRedshiftServerless: mustGetEnv(t, rssDiscoveryRoleARNEnv),
		types.AWSMatcherRedshift:           mustGetEnv(t, redshiftDiscoveryRoleARNEnv),
	} {
		matchers = append(matchers, types.AWSMatcher{
			Types: []string{matcherType},
			Tags: types.Labels{
				// This label should not match.
				"env": {"tag_not_found"},
			},
			Regions: []string{awsRegion},
			AssumeRole: &types.AssumeRole{
				RoleARN: assumeRoleARN,
			},
		})
	}

	cluster := createTeleportCluster(t,
		withSingleProxyPort(t),
		withDiscoveryService(t, "db-e2e-test", matchers...),
	)

	// Get the auth server.
	authC := cluster.Process.GetAuthServer()
	// Wait for the discovery service to not create a database resource
	// because the database does not match the selectors.
	require.Never(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		databases, err := authC.GetDatabases(ctx)
		return err == nil && len(databases) != 0
	}, 2*time.Minute, 10*time.Second, "discovery service incorrectly created a database")
}

const (
	waitForConnTimeout = 90 * time.Second
	connRetryTick      = 10 * time.Second
)

// postgresConnTestFn tests connection to a postgres database via proxy web
// multiplexer.
func postgresConnTest(t *testing.T, cluster *helpers.TeleInstance, user string, route tlsca.RouteToDatabase, query string) {
	t.Helper()
	var pgConn *pgconn.PgConn
	waitForDBConnection(t, func(ctx context.Context) error {
		var err error
		pgConn, err = postgres.MakeTestClient(ctx, common.TestClientConfig{
			AuthClient:      cluster.GetSiteAPI(cluster.Secrets.SiteName),
			AuthServer:      cluster.Process.GetAuthServer(),
			Address:         cluster.Web,
			Cluster:         cluster.Secrets.SiteName,
			Username:        user,
			RouteToDatabase: route,
		})
		return err
	})
	execPGTestQuery(t, pgConn, query)
}

// postgresLocalProxyConnTest tests connection to a postgres database via
// local proxy tunnel.
func postgresLocalProxyConnTest(t *testing.T, cluster *helpers.TeleInstance, user string, route tlsca.RouteToDatabase, query string) {
	t.Helper()
	lp := startLocalALPNProxy(t, user, cluster, route)

	pgconnConfig, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%v/", lp.GetAddr()))
	require.NoError(t, err)
	pgconnConfig.User = route.Username
	pgconnConfig.Database = route.Database
	var pgConn *pgconn.PgConn
	waitForDBConnection(t, func(ctx context.Context) error {
		var err error
		pgConn, err = pgconn.ConnectConfig(ctx, pgconnConfig)
		return err
	})
	execPGTestQuery(t, pgConn, query)
}

func execPGTestQuery(t *testing.T, conn *pgconn.PgConn, query string) {
	t.Helper()
	defer func() {
		// dont wait forever to gracefully terminate.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Disconnect.
		require.NoError(t, conn.Close(ctx))
	}()

	// dont wait forever on the exec.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute a query.
	results, err := conn.Exec(ctx, query).ReadAll()
	require.NoError(t, err)
	for i, r := range results {
		require.NoError(t, r.Err, "error in result %v", i)
	}
}

// mysqlLocalProxyConnTest tests connection to a MySQL database via
// local proxy tunnel.
func mysqlLocalProxyConnTest(t *testing.T, cluster *helpers.TeleInstance, user string, route tlsca.RouteToDatabase, query string) {
	t.Helper()
	lp := startLocalALPNProxy(t, user, cluster, route)

	var conn *mysqlclient.Conn
	waitForDBConnection(t, func(ctx context.Context) error {
		var err error
		var nd net.Dialer
		conn, err = mysqlclient.ConnectWithDialer(ctx, "tcp",
			lp.GetAddr(),
			route.Username,
			"", /*no password*/
			route.Database,
			nd.DialContext,
		)
		return err
	})
	defer func() {
		// Disconnect.
		require.NoError(t, conn.Close())
	}()

	// Execute a query.
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	_, err := conn.Execute(query)
	require.NoError(t, err)
}

// startLocalALPNProxy starts local ALPN proxy for the specified database.
func startLocalALPNProxy(t *testing.T, user string, cluster *helpers.TeleInstance, route tlsca.RouteToDatabase) *alpnproxy.LocalProxy {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proto, err := alpncommon.ToALPNProtocol(route.Protocol)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxyNetAddr, err := cluster.Process.ProxyWebAddr()
	require.NoError(t, err)

	authSrv := cluster.Process.GetAuthServer()
	tlsCert := generateClientDBCert(t, authSrv, user, route)

	proxy, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    proxyNetAddr.String(),
		Protocols:          []alpncommon.Protocol{proto},
		InsecureSkipVerify: true,
		Listener:           listener,
		ParentContext:      ctx,
		Cert:               tlsCert,
	})
	require.NoError(t, err)

	go proxy.Start(ctx)
	t.Cleanup(func() {
		_ = proxy.Close()
	})

	return proxy
}

// generateClientDBCert creates a test db cert for the given user and database.
func generateClientDBCert(t *testing.T, authSrv *auth.Server, user string, route tlsca.RouteToDatabase) tls.Certificate {
	t.Helper()
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	clusterName, err := authSrv.GetClusterName(context.TODO())
	require.NoError(t, err)

	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)

	clientCert, err := authSrv.GenerateDatabaseTestCert(
		auth.DatabaseTestCertRequest{
			PublicKey:       publicKeyPEM,
			Cluster:         clusterName.GetClusterName(),
			Username:        user,
			RouteToDatabase: route,
		})
	require.NoError(t, err)

	tlsCert, err := keys.TLSCertificateForSigner(key, clientCert)
	require.NoError(t, err)
	return tlsCert
}

func waitForDatabases(t *testing.T, auth *service.TeleportProcess, wantNames ...string) {
	t.Helper()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		databases, err := auth.GetAuthServer().GetDatabases(ctx)
		assert.NoError(t, err)

		// map the registered "db" resource names.
		seen := map[string]struct{}{}
		for _, db := range databases {
			seen[db.GetName()] = struct{}{}
		}
		for _, name := range wantNames {
			assert.Contains(t, seen, name)
		}
	}, 3*time.Minute, 3*time.Second, "waiting for the discovery service to create db resources")
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		servers, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
		assert.NoError(t, err)

		// map the registered "db_server" resource names.
		seen := map[string]struct{}{}
		for _, s := range servers {
			seen[s.GetName()] = struct{}{}
		}
		for _, name := range wantNames {
			assert.Contains(t, seen, name)
		}
	}, 1*time.Minute, time.Second, "waiting for the database service to heartbeat the databases")
}

// dbUserLogin contains common info needed to connect as a db user via
// password auth.
type dbUserLogin struct {
	username string
	password string
	address  string
	port     int
}

func connectPostgres(t *testing.T, ctx context.Context, info dbUserLogin, dbName string) *pgConn {
	pgCfg, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s:%d/?sslmode=verify-full", info.address, info.port))
	require.NoError(t, err)
	pgCfg.User = info.username
	pgCfg.Password = info.password
	pgCfg.Database = dbName
	pgCfg.TLSConfig = &tls.Config{
		ServerName: info.address,
		RootCAs:    awsCertPool.Clone(),
	}

	conn, err := pgx.ConnectConfig(ctx, pgCfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close(ctx)
	})
	return &pgConn{
		logger: utils.NewSlogLoggerForTests().With("test_name", t.Name()),
		Conn:   conn,
	}
}

// secretPassword is used to unmarshal an AWS Secrets Manager
// user password secret.
type secretPassword struct {
	Password string `json:"password"`
}

// getMasterUserPassword is a helper that fetches a db master user and password
// from AWS Secrets Manager.
func getMasterUserPassword(t *testing.T, ctx context.Context, secretID string) string {
	t.Helper()
	secretVal := getSecretValue(t, ctx, secretID)
	require.NotNil(t, secretVal.SecretString)
	var secret secretPassword
	if err := json.Unmarshal([]byte(*secretVal.SecretString), &secret); err != nil {
		// being paranoid. I don't want to leak the secret string in test error
		// logs.
		require.FailNow(t, "error unmarshaling secret string")
	}
	if len(secret.Password) == 0 {
		require.FailNow(t, "empty master user secret string")
	}
	return secret.Password
}

func getSecretValue(t *testing.T, ctx context.Context, secretID string) secretsmanager.GetSecretValueOutput {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(mustGetEnv(t, awsRegionEnv)),
	)
	require.NoError(t, err)

	secretsClt := secretsmanager.NewFromConfig(cfg)
	secretVal, err := secretsClt.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretID,
	})
	require.NoError(t, err)
	require.NotNil(t, secretVal)
	return *secretVal
}

// pgConn wraps a [pgx.Conn] and adds retries to all Exec calls.
type pgConn struct {
	logger *slog.Logger
	*pgx.Conn
}

func (c *pgConn) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	var out pgconn.CommandTag
	err := withRetry(ctx, c.logger, func() error {
		var err error
		out, err = c.Conn.Exec(ctx, sql, args...)
		return trace.Wrap(err)
	})
	c.logger.InfoContext(ctx, "Executed sql statement",
		"sql", sql,
		"error", err,
	)
	return out, trace.Wrap(err)
}

// withRetry runs a given func a finite number of times until it returns nil
// error or the given context is done.
func withRetry(ctx context.Context, log *slog.Logger, f func() error) error {
	linear, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  0,
		Step:   500 * time.Millisecond,
		Max:    5 * time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// retry a finite number of times before giving up.
	const retries = 10
	for range retries {
		err := f()
		if err == nil {
			return nil
		}

		if isRetryable(err) {
			log.DebugContext(ctx, "operation failed, retrying", "error", err)
		} else {
			return trace.Wrap(err)
		}

		linear.Inc()
		select {
		case <-linear.After():
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
	return trace.Wrap(err, "too many retries")
}

// isRetryable returns true if an error can be retried.
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	err = trace.Unwrap(err)
	if errors.As(err, &pgErr) {
		// https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
		switch pgErr.Code {
		case pgerrcode.DeadlockDetected, pgerrcode.SerializationFailure,
			pgerrcode.UniqueViolation, pgerrcode.ExclusionViolation:
			return true
		}
	}
	// Redshift reports this with a vague SQLSTATE XX000, which is the internal
	// error code, but this is a serialization error that rolls back the
	// transaction, so it should be retried.
	if strings.Contains(err.Error(), "conflict with concurrent transaction") {
		return true
	}
	return pgconn.SafeToRetry(err)
}

func waitForDBConnection(t *testing.T, connectFn func(context.Context) error) {
	t.Helper()
	// retry for a while, the database service might need time to give itself
	// IAM permissions.
	waitForSuccess(t, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), connRetryTick)
		defer cancel()
		return connectFn(ctx)
	}, waitForConnTimeout, connRetryTick, "connecting to database")
}

// waitForSuccess is a test helper that wraps require.EventuallyWithT but runs
// the given fn first to avoid waiting for the first timer tick.
func waitForSuccess(t *testing.T, fn func() error, waitDur, tick time.Duration, msgAndArgs ...any) {
	t.Helper()
	// EventuallyWithT waits for the first tick before it makes the first
	// attempt, so to speed things up we check for fn success first.
	if err := fn(); err == nil {
		return
	}
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.NoError(t, fn())
	}, waitDur, tick, msgAndArgs...)
}
