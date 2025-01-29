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
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	memorydb "github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/redis/connection"
	"github.com/gravitational/teleport/lib/srv/db/redis/protocol"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
	// AWSClients is an SDK client provider.
	// This field is only exported so it can be overridden in integration tests.
	AWSClients AWSClientProvider
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
	// clientMessageRead indicates processing client messages has started.
	clientMessageRead bool
}

// AWSClientProvider provides AWS service API clients.
type AWSClientProvider interface {
	// GetElastiCacheClient provides an [ElastiCacheClient].
	GetElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) ElastiCacheClient
	// GetMemoryDBClient provides an [MemoryDBClient].
	GetMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) MemoryDBClient
}

// ElastiCacheClient is a subset of the AWS ElastiCache API.
type ElastiCacheClient interface {
	elasticache.DescribeUsersAPIClient
}

// MemoryDBClient is a subset of the AWS MemoryDB API.
type MemoryDBClient interface {
	memorydb.DescribeUsersAPIClient
}

type defaultAWSClients struct{}

func (defaultAWSClients) GetElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) ElastiCacheClient {
	return elasticache.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) GetMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) MemoryDBClient {
	return memorydb.NewFromConfig(cfg, optFns...)
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.clientReader = redis.NewReader(clientConn)
	e.sessionCtx = sessionCtx
	if e.AWSClients == nil {
		e.AWSClients = defaultAWSClients{}
	}

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

	// If the first message is a HELLO test, do not return authentication
	// errors to the HELLO command as it can be swallowed by the client as part
	// of its fallback mechanism. First return the unknown command error then
	// send the authentication errors to the next incoming command (usually
	// AUTH).
	//
	// Background: The HELLO test is used for establishing the RESP3 protocol
	// but Teleport currently only supports RESP2. The client generally
	// fallbacks to RESP2 when they receive an unknown command error for the
	// HELLO message.
	e.maybeHandleFirstHello()

	if err := e.sendToClient(redisErr); err != nil {
		e.Log.ErrorContext(e.Context, "Failed to send message to the client.", "error", err)
		return
	}
}

// maybeHandleFirstHello replies an unknown command error to the client if the
// first message is a HELLO test.
func (e *Engine) maybeHandleFirstHello() {
	// Return if not the first message.
	if e.clientMessageRead {
		return
	}

	// Let's not wait forever for the HELLO message.
	ctx, cancel := context.WithTimeout(e.Context, 10*time.Second)
	defer cancel()

	cmd, err := e.readClientCmd(ctx)
	if err != nil {
		e.Log.ErrorContext(e.Context, "Failed to read first client message.", "error", err)
		return
	}

	// Return if not a HELLO.
	if strings.ToLower(cmd.Name()) != helloCmd {
		return
	}
	response := protocol.MakeUnknownCommandErrorForCmd(cmd)
	if err := e.sendToClient(response); err != nil {
		e.Log.ErrorContext(e.Context, "Failed to send message to the client.", "error", err)
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
			e.Log.ErrorContext(e.Context, "Failed to close Redis connection.", "error", err)
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
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
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
		credenialsProvider, err := e.createCredentialsProvider(ctx, sessionCtx, username, password)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		redisClient, err := newClient(ctx, connectionOptions, tlsConfig, credenialsProvider)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return redisClient, nil
	}, nil
}

// createCredentialsProvider creates a callback function that provides username
// and password.
// This function may return nil, nil as nil credenialsProvider is valid.
func (e *Engine) createCredentialsProvider(ctx context.Context, sessionCtx *common.Session, username, password string) (fetchCredentialsFunc, error) {
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
		credFetchFn := managedUserCredFetchFunc(sessionCtx, e.Users)
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
		return nil, nil
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
			e.Log.DebugContext(e.Context, "IAM Auth is enabled.", "user", sessionCtx.DatabaseUser, "database", sessionCtx.Database.GetName())
		}
	}()
	// check if the db supports IAM auth. If we get an error, assume the db does
	// support IAM auth. False positives just incur an extra API call.
	if ok, err := checkDBSupportsIAMAuth(sessionCtx.Database); err != nil {
		e.Log.DebugContext(ctx, "Assuming database supports IAM auth.", "database", sessionCtx.Database.GetName())
	} else if !ok {
		return false
	}
	dbUser := sessionCtx.DatabaseUser
	ok, err := e.checkUserIAMAuthIsEnabled(ctx, sessionCtx, dbUser)
	if err != nil {
		e.Log.DebugContext(e.Context, "Assuming IAM auth is not enabled for user.", "user", dbUser, "error", err)
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
func (e *Engine) checkUserIAMAuthIsEnabled(ctx context.Context, sessionCtx *common.Session, username string) (bool, error) {
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeElastiCache:
		return e.checkElastiCacheUserIAMAuthIsEnabled(ctx, sessionCtx.Database.GetAWS(), username)
	case types.DatabaseTypeMemoryDB:
		return e.checkMemoryDBUserIAMAuthIsEnabled(ctx, sessionCtx.Database.GetAWS(), username)
	default:
		return false, nil
	}
}

func (e *Engine) checkElastiCacheUserIAMAuthIsEnabled(ctx context.Context, awsMeta types.AWS, username string) (bool, error) {
	awsCfg, err := e.AWSConfigProvider.GetConfig(ctx, awsMeta.Region,
		awsconfig.WithAssumeRole(awsMeta.AssumeRoleARN, awsMeta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return false, trace.Wrap(err)
	}
	client := e.AWSClients.GetElastiCacheClient(awsCfg)
	// For IAM-enabled ElastiCache users, the username and user id properties
	// must be identical.
	// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth-iam.html#auth-iam-limits
	input := elasticache.DescribeUsersInput{UserId: aws.String(username)}
	out, err := client.DescribeUsers(ctx, &input)
	if err != nil {
		return false, trace.Wrap(libaws.ConvertRequestFailureErrorV2(err))
	}
	if len(out.Users) < 1 || out.Users[0].Authentication == nil {
		return false, nil
	}
	authType := out.Users[0].Authentication.Type
	return ectypes.AuthenticationTypeIam == authType, nil
}

func (e *Engine) checkMemoryDBUserIAMAuthIsEnabled(ctx context.Context, awsMeta types.AWS, username string) (bool, error) {
	awsCfg, err := e.AWSConfigProvider.GetConfig(ctx, awsMeta.Region,
		awsconfig.WithAssumeRole(awsMeta.AssumeRoleARN, awsMeta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return false, trace.Wrap(err)
	}
	client := e.AWSClients.GetMemoryDBClient(awsCfg)
	input := memorydb.DescribeUsersInput{UserName: aws.String(username)}
	out, err := client.DescribeUsers(ctx, &input)
	if err != nil {
		return false, trace.Wrap(libaws.ConvertRequestFailureErrorV2(err))
	}
	if len(out.Users) < 1 || out.Users[0].Authentication == nil {
		return false, nil
	}
	authType := out.Users[0].Authentication.Type
	return memorydbtypes.AuthenticationTypeIam == authType, nil
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
	e.clientMessageRead = true

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
	var redisError redis.RedisError
	return errors.As(err, &redisError)
}

// isTeleportErr returns true if error comes from Teleport itself.
func isTeleportErr(err error) bool {
	var error trace.Error
	return errors.As(err, &error)
}

// driverLogger implements go-redis driver's internal logger using slog and
// logs everything at TRACE level.
type driverLogger struct {
	*slog.Logger
}

func (l *driverLogger) Printf(ctx context.Context, format string, v ...any) {
	if !l.Logger.Enabled(ctx, logutils.TraceLevel) {
		return
	}

	//nolint:sloglint // Allow non-static messages
	l.Logger.Log(ctx, logutils.TraceLevel, fmt.Sprintf(format, v...))
}

func init() {
	redis.SetLogger(&driverLogger{
		Logger: slog.With(teleport.ComponentKey, "go-redis"),
	})
}
