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

package redis

import (
	"bytes"
	"context"
	"errors"
	"net"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/redis/connection"
	"github.com/gravitational/teleport/lib/srv/db/redis/protocol"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new Redis engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// redisClientFactoryFn is a prototype that takes Redis username and password and return a new
// Redis client.
type redisClientFactoryFn func(username, password string) (redis.UniversalClient, error)

// Engine implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// clientReader is a go-redis wrapper for Redis client connection.
	clientReader *redis.Reader
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// newClient returns a new client connection
	newClient redisClientFactoryFn
	// redisClient is a current connection to Redis server.
	redisClient redis.UniversalClient
	// awsIAMAuthSupported is the saved result of isAWSIAMAuthSupported.
	awsIAMAuthSupported *bool
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.clientReader = redis.NewReader(clientConn)
	e.sessionCtx = sessionCtx

	// Use Redis default user named "default" if a user is not provided.
	if e.sessionCtx.DatabaseUser == "" {
		e.sessionCtx.DatabaseUser = defaults.DefaultRedisUsername
	}

	return nil
}

// authorizeConnection does authorization check for Redis connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context) error {
	if err := e.checkDefaultUserRequired(); err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := e.sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     e.sessionCtx.Database,
		DatabaseUser: e.sessionCtx.DatabaseUser,
		DatabaseName: e.sessionCtx.DatabaseName,
	})
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

// checkDefaultUserRequired checks if the session db user is the "default" Redis
// user, and checks if the session db user must be the default user.
// When the session db user is not "default", but it's required to be "default",
// return an error.
func (e *Engine) checkDefaultUserRequired() error {
	// When the db user is already "default", there's no need to check if it's
	// required to be "default".
	if defaults.DefaultRedisUsername == e.sessionCtx.DatabaseUser {
		return nil
	}
	if e.sessionCtx.Database.IsAzure() {
		return trace.AccessDenied("access denied to non-default db user: "+
			"Azure Cache for Redis requires authentication as default user %q",
			defaults.DefaultRedisUsername)
	}
	return nil
}

// SendError sends error message to connected client.
func (e *Engine) SendError(redisErr error) {
	if redisErr == nil || utils.IsOKNetworkError(redisErr) {
		return
	}

	if err := e.sendToClient(redisErr); err != nil {
		e.Log.Errorf("Failed to send message to the client: %v.", err)
		return
	}
}

// sendToClient sends a command to connected Redis client.
func (e *Engine) sendToClient(vals interface{}) error {
	if vals == nil {
		return nil
	}

	buf := &bytes.Buffer{}
	wr := redis.NewWriter(buf)

	if err := protocol.WriteCmd(wr, vals); err != nil {
		return trace.BadParameter("failed to convert error to a message: %v", err)
	}

	if _, err := e.clientConn.Write(buf.Bytes()); err != nil {
		return trace.ConnectionProblem(err, "failed to send message to the client")
	}

	return nil
}

// HandleConnection is responsible for connecting to a Redis instance/cluster.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)

	// Check that the user has access to the database.
	err := e.authorizeConnection(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Initialize newClient factory function with current connection state.
	e.newClient, err = e.getNewClientFn(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create new client without username or password. Those will be added
	// when we receive AUTH command (e.g. self-hosted), or they can be
	// fetched by the OnConnect callback
	// (e.g. ElastiCache managed users, IAM auth, or Azure access key).
	e.redisClient, err = e.newClient("", "")
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := e.redisClient.Close(); err != nil {
			e.Log.Errorf("Failed to close Redis connection: %v.", err)
		}
	}()

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	observe()

	if err := e.process(ctx, sessionCtx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// getNewClientFn returns a partial Redis client factory function.
func (e *Engine) getNewClientFn(ctx context.Context, sessionCtx *common.Session) (redisClientFactoryFn, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set default mode. Default mode can be overridden by URI parameters.
	defaultMode := connection.Standalone
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeElastiCache:
		if sessionCtx.Database.GetAWS().ElastiCache.EndpointType == apiawsutils.ElastiCacheConfigurationEndpoint {
			defaultMode = connection.Cluster
		}

	case types.DatabaseTypeMemoryDB:
		if sessionCtx.Database.GetAWS().MemoryDB.EndpointType == apiawsutils.MemoryDBClusterEndpoint {
			defaultMode = connection.Cluster
		}

	case types.DatabaseTypeAzure:
		// "OSSCluster" requires client to use the OSS Cluster mode.
		//
		// https://learn.microsoft.com/en-us/azure/azure-cache-for-redis/quickstart-create-redis-enterprise#clustering-policy
		if sessionCtx.Database.GetAzure().Redis.ClusteringPolicy == azure.RedisEnterpriseClusterPolicyOSS {
			defaultMode = connection.Cluster
		}
	}

	connectionOptions, err := connection.ParseRedisAddressWithDefaultMode(sessionCtx.Database.GetURI(), defaultMode)
	if err != nil {
		return nil, trace.BadParameter("Redis connection string is incorrect %q: %v", sessionCtx.Database.GetURI(), err)
	}

	return func(username, password string) (redis.UniversalClient, error) {
		onConnect, err := e.createOnClientConnectFunc(ctx, sessionCtx, username, password)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		redisClient, err := newClient(ctx, connectionOptions, tlsConfig, onConnect)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return redisClient, nil
	}, nil
}

// createOnClientConnectFunc creates a callback function that is called after a
// successful client connection with the Redis server.
func (e *Engine) createOnClientConnectFunc(ctx context.Context, sessionCtx *common.Session, username, password string) (onClientConnectFunc, error) {
	switch {
	// If password is provided by client.
	case password != "":
		return authWithPasswordOnConnect(username, password), nil

	// Azure databases authenticate via access keys.
	case sessionCtx.Database.IsAzure():
		credFetchFn := azureAccessKeyFetchFunc(sessionCtx, e.Auth)
		return fetchCredentialsOnConnect(e.Context, sessionCtx, e.Audit, credFetchFn), nil

	// If database user is one of managed users (AWS only).
	//
	// Teleport managed users can have their passwords rotated during a
	// database session. Fetching an user's password on each new connection
	// ensures the correct password is used for each shard connection when
	// Redis is in cluster mode.
	case slices.Contains(sessionCtx.Database.GetManagedUsers(), sessionCtx.DatabaseUser):
		credFetchFn := managedUserCredFetchFunc(sessionCtx, e.Auth, e.Users)
		return fetchCredentialsOnConnect(e.Context, sessionCtx, e.Audit, credFetchFn), nil

	// AWS ElastiCache has limited support for IAM authentication.
	// See: https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth-iam.html#auth-iam-limits
	// So we must check that the database supports IAM auth and that the
	// ElastiCache user has IAM auth enabled.
	// Same applies for AWS MemoryDB.
	case e.isAWSIAMAuthSupported(ctx, sessionCtx):
		credFetchFn, err := awsIAMTokenFetchFunc(sessionCtx, e.Auth)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fetchCredentialsOnConnect(e.Context, sessionCtx, e.Audit, credFetchFn), nil

	default:
		return noopOnConnect, nil
	}
}

// isAWSIAMAuthSupported returns whether AWS IAM auth is supported for the
// database and the database user.
func (e *Engine) isAWSIAMAuthSupported(ctx context.Context, sessionCtx *common.Session) (res bool) {
	if e.awsIAMAuthSupported != nil {
		return *e.awsIAMAuthSupported
	}
	defer func() {
		// cache result to avoid API calls on each new instance connection.
		e.awsIAMAuthSupported = &res
		if res {
			e.Log.Debugf("IAM Auth is enabled for user %q in database %q.", sessionCtx.DatabaseUser, sessionCtx.Database.GetName())
		}
	}()
	// check if the db supports IAM auth. If we get an error, assume the db does
	// support IAM auth. False positives just incur an extra API call.
	if ok, err := checkDBSupportsIAMAuth(sessionCtx.Database); err != nil {
		e.Log.WithError(err).Debugf("Assuming database %s supports IAM auth.",
			sessionCtx.Database.GetName())
	} else if !ok {
		return false
	}
	dbUser := sessionCtx.DatabaseUser
	ok, err := checkUserIAMAuthIsEnabled(ctx, sessionCtx, e.CloudClients, dbUser)
	if err != nil {
		e.Log.WithError(err).Debugf("Assuming IAM auth is not enabled for user %s.",
			dbUser)
		return false
	}
	return ok
}

// checkDBSupportsIAMAuth returns whether the given database is an ElastiCache
// or MemoryDB database that supports IAM auth.
// AWS ElastiCache Redis/MemoryDB supports IAM auth for redis version 7+.
func checkDBSupportsIAMAuth(database types.Database) (bool, error) {
	switch database.GetType() {
	case types.DatabaseTypeElastiCache:
		return iam.CheckElastiCacheSupportsIAMAuth(database)
	case types.DatabaseTypeMemoryDB:
		return iam.CheckMemoryDBSupportsIAMAuth(database)
	default:
		return false, nil
	}
}

// checkUserIAMAuthIsEnabled returns whether a given ElastiCache or MemoryDB
// user has IAM auth enabled.
func checkUserIAMAuthIsEnabled(ctx context.Context, sessionCtx *common.Session, clients cloud.Clients, username string) (bool, error) {
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeElastiCache:
		return checkElastiCacheUserIAMAuthIsEnabled(ctx, clients, sessionCtx.Database.GetAWS(), username)
	case types.DatabaseTypeMemoryDB:
		return checkMemoryDBUserIAMAuthIsEnabled(ctx, clients, sessionCtx.Database.GetAWS(), username)
	default:
		return false, nil
	}
}

func checkElastiCacheUserIAMAuthIsEnabled(ctx context.Context, clients cloud.Clients, awsMeta types.AWS, username string) (bool, error) {
	client, err := clients.GetAWSElastiCacheClient(ctx, awsMeta.Region,
		cloud.WithAssumeRoleFromAWSMeta(awsMeta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return false, trace.Wrap(err)
	}
	// For IAM-enabled ElastiCache users, the username and user id properties
	// must be identical.
	// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth-iam.html#auth-iam-limits
	input := elasticache.DescribeUsersInput{UserId: aws.String(username)}
	out, err := client.DescribeUsersWithContext(ctx, &input)
	if err != nil {
		return false, trace.Wrap(libaws.ConvertRequestFailureError(err))
	}
	if len(out.Users) < 1 || out.Users[0].Authentication == nil {
		return false, nil
	}
	authType := aws.StringValue(out.Users[0].Authentication.Type)
	return elasticache.AuthenticationTypeIam == authType, nil
}

func checkMemoryDBUserIAMAuthIsEnabled(ctx context.Context, clients cloud.Clients, awsMeta types.AWS, username string) (bool, error) {
	client, err := clients.GetAWSMemoryDBClient(ctx, awsMeta.Region,
		cloud.WithAssumeRoleFromAWSMeta(awsMeta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return false, trace.Wrap(err)
	}
	input := memorydb.DescribeUsersInput{UserName: aws.String(username)}
	out, err := client.DescribeUsersWithContext(ctx, &input)
	if err != nil {
		return false, trace.Wrap(libaws.ConvertRequestFailureError(err))
	}
	if len(out.Users) < 1 || out.Users[0].Authentication == nil {
		return false, nil
	}
	authType := aws.StringValue(out.Users[0].Authentication.Type)
	return memorydb.AuthenticationTypeIam == authType, nil
}

// reconnect closes the current Redis server connection and creates a new one pre-authenticated
// with provided username and password.
func (e *Engine) reconnect(username, password string) (redis.UniversalClient, error) {
	err := e.redisClient.Close()
	if err != nil {
		return nil, trace.Wrap(err, "failed to close Redis connection")
	}

	e.redisClient, err = e.newClient(username, password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return e.redisClient, nil
}

// process is the main processing function for Redis. It reads commands from connected client and passes them to
// a Redis instance. This function returns when a server closes a connection or in case of connection error.
func (e *Engine) process(ctx context.Context, sessionCtx *common.Session) error {
	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(e.sessionCtx.Database)

	for {
		// Read commands from connected client.
		cmd, err := e.readClientCmd(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		msgFromClient.Inc()

		// send valid commands to Redis instance/cluster.
		err = e.processCmd(ctx, cmd)
		// go-redis returns some errors as err and some as cmd.Err().
		// Function below maps errors that should be returned to the
		// client as value or return them as err if we should terminate
		// the session.
		value, err := e.processServerResponse(cmd, err, sessionCtx)
		if err != nil {
			return trace.Wrap(err)
		}

		msgFromServer.Inc()

		// Send response back to the client.
		if err := e.sendToClient(value); err != nil {
			return trace.Wrap(err)
		}
	}
}

// readClientCmd reads commands from connected Redis client.
func (e *Engine) readClientCmd(ctx context.Context) (*redis.Cmd, error) {
	cmd := &redis.Cmd{}
	if err := cmd.ReadReply(e.clientReader); err != nil {
		return nil, trace.Wrap(err)
	}

	val, ok := cmd.Val().([]interface{})
	if !ok {
		return nil, trace.BadParameter("failed to cast Redis value to a slice, got %T", cmd.Val())
	}

	return redis.NewCmd(ctx, val...), nil
}

// processServerResponse takes server response and an error returned from go-redis and returns
// "terminal" errors as second value (connection should be terminated when this happens)
// or returns error/value as the first value. Then value should be sent back to
// the client without terminating the connection.
func (e *Engine) processServerResponse(cmd *redis.Cmd, err error, sessionCtx *common.Session) (interface{}, error) {
	value, cmdErr := cmd.Result()
	if err == nil {
		// If the server didn't return any error use cmd.Err() as server error.
		err = cmdErr
	}

	switch {
	case e.isIAMAuthError(err):
		return common.ConvertConnectError(trace.AccessDenied(err.Error()), sessionCtx), nil
	case isRedisError(err):
		// Redis errors should be returned to the client.
		return err, nil
	case isTeleportErr(err):
		// Teleport errors should be returned to the client.
		return err, nil
	case errors.Is(err, context.DeadlineExceeded):
		switch sessionCtx.Database.GetType() {
		// Special message for ElastiCache servers without TLS enabled.
		case types.DatabaseTypeElastiCache:
			if !sessionCtx.Database.GetAWS().ElastiCache.TransitEncryptionEnabled {
				return nil, trace.ConnectionProblem(err, "Connection timeout on ElastiCache database. Please verify if in-transit encryption is enabled on the server.")
			}

		// Special message for MemoryDB servers without TLS enabled.
		case types.DatabaseTypeMemoryDB:
			if !sessionCtx.Database.GetAWS().MemoryDB.TLSEnabled {
				return nil, trace.ConnectionProblem(err, "Connection timeout on MemoryDB database. Please verify if in-transit encryption is enabled on the server.")
			}
		}

		// Do not return Deadline Exceeded to the client as it's not very self-explanatory.
		// Return "connection timeout" as this is what most likely happened.
		return nil, trace.ConnectionProblem(err, "connection timeout")
	case utils.IsConnectionRefused(err):
		// "connection refused" is returned when we fail to connect to the DB or a connection
		// has been lost. Replace with more meaningful error.
		return nil, trace.ConnectionProblem(err, "failed to connect to the target database")
	default:
		// Return value and the error. If the error is not nil we will close the connection.
		return value, common.ConvertConnectError(err, sessionCtx)
	}
}

// isIAMAuthError detects an ElastiCache IAM auth error.
func (e *Engine) isIAMAuthError(err error) bool {
	if err == nil || e.awsIAMAuthSupported == nil || !*e.awsIAMAuthSupported {
		return false
	}
	return strings.Contains(err.Error(), "WRONGPASS")
}

// isRedisError returns true is error comes from Redis, ex, nil, bad command, etc.
func isRedisError(err error) bool {
	_, ok := err.(redis.RedisError)
	return ok
}

// isTeleportErr returns true if error comes from Teleport itself.
func isTeleportErr(err error) bool {
	_, ok := err.(trace.Error)
	return ok
}

// driverLogger implements go-redis driver's internal logger using logrus and
// logs everything at TRACE level.
type driverLogger struct {
	*logrus.Entry
}

func (l *driverLogger) Printf(_ context.Context, format string, v ...any) {
	l.Entry.Tracef(format, v...)
}

func init() {
	redis.SetLogger(&driverLogger{
		Entry: logrus.WithField(teleport.ComponentKey, "go-redis"),
	})
}
