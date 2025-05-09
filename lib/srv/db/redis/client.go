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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/redis/connection"
	"github.com/gravitational/teleport/lib/srv/db/redis/protocol"
)

// Commands with additional processing in Teleport when using cluster mode.
const (
	dbsizeCmd   = "dbsize"
	keysCmd     = "keys"
	mgetCmd     = "mget"
	flushallCmd = "flushall"
	flushdbCmd  = "flushdb"
	scriptCmd   = "script"
)

// Overridden subcommands for Redis SCRIPT command.
const (
	scriptLoadSubcmd   = "load"
	scriptExistsSubcmd = "exists"
	scriptFLushSubcmd  = "flush"
)

// List of unsupported commands in cluster mode.
const (
	aclCmd        = "acl"
	askingCmd     = "asking"
	clientCmd     = "client"
	clusterCmd    = "cluster"
	configCmd     = "config"
	debugCmd      = "debug"
	execCmd       = "exec"
	infoCmd       = "info"
	latencyCmd    = "latency"
	memoryCmd     = "memory"
	migrateCmd    = "migrate"
	moduleCmd     = "module"
	monitorCmd    = "monitor"
	multiCmd      = "multi"
	pfdebugCmd    = "pfdebug"
	pfselftestCmd = "pfselftest"
	psyncCmd      = "psync"
	readonlyCmd   = "readonly"
	readwriteCmd  = "readwrite"
	replconfCmd   = "replconf"
	replicaofCmd  = "replicaof"
	roleCmd       = "role"
	scanCmd       = "scan"
	shutdownCmd   = "shutdown"
	slaveofCmd    = "slaveof"
	slowlogCmd    = "slowlog"
	syncCmd       = "sync"
	timeCmd       = "time"
	waitCmd       = "wait"
	watchCmd      = "watch"
)

const (
	// aclWhoami is a subcommand of "acl" that requires special handling.
	aclWhoami = "whoami"
	// protocolV2 defines the RESP protocol v2 that Teleport uses.
	protocolV2 = 2
)

// clusterClient is a wrapper around redis.ClusterClient
type clusterClient struct {
	redis.ClusterClient
}

// newClient creates a new Redis client based on given ConnectionMode. If connection mode is not supported
// an error is returned.
func newClient(ctx context.Context, connectionOptions *connection.Options, tlsConfig *tls.Config, credentialsProvider fetchCredentialsFunc) (redis.UniversalClient, error) {
	connectionAddr := getHostPort(connectionOptions)
	// TODO(jakub): Investigate Redis Sentinel.
	switch connectionOptions.Mode {
	case connection.Standalone:
		return redis.NewClient(&redis.Options{
			Addr:                       connectionAddr,
			TLSConfig:                  tlsConfig,
			CredentialsProviderContext: credentialsProvider,
			Protocol:                   protocolV2,
			DisableIndentity:           true,
		}), nil
	case connection.Cluster:
		client := &clusterClient{
			ClusterClient: *redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:                      []string{connectionAddr},
				TLSConfig:                  tlsConfig,
				CredentialsProviderContext: credentialsProvider,
				Protocol:                   protocolV2,
				DisableIndentity:           true,
			}),
		}
		// Load cluster information.
		client.ReloadState(ctx)

		return client, nil
	default:
		// We've checked that while validating the config, but checking again can help with regression.
		return nil, trace.BadParameter("incorrect connection mode %s", connectionOptions.Mode)
	}
}

// fetchCredentialsFunc fetches credentials for a new connection.
type fetchCredentialsFunc func(ctx context.Context) (username, password string, err error)

// authWithPasswordOnConnect returns an fetchCredentialsFunc that sends "auth"
// with provided username and password.
func authWithPasswordOnConnect(username, password string) fetchCredentialsFunc {
	return func(ctx context.Context) (string, string, error) {
		return username, password, nil
	}
}

// fetchCredentialsOnConnect returns an fetchCredentialsFunc that does an
// authorization check, calls a provided credential fetcher callback func,
// then logs an AUTH query to the audit log once and and uses the credentials to
// auth a new connection.
func fetchCredentialsOnConnect(closeCtx context.Context, sessionCtx *common.Session, audit common.Audit, fetchCreds fetchCredentialsFunc) fetchCredentialsFunc {
	// audit log one time, to avoid excessive audit logs from reconnects.
	var auditOnce sync.Once
	return func(ctx context.Context) (string, string, error) {
		err := sessionCtx.Checker.CheckAccess(sessionCtx.Database,
			services.AccessState{MFAVerified: true},
			role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
				Database:     sessionCtx.Database,
				DatabaseUser: sessionCtx.DatabaseUser,
				DatabaseName: sessionCtx.DatabaseName,
			})...)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		username, password, err := fetchCreds(ctx)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		auditOnce.Do(func() {
			var query string
			if username == "" {
				query = "AUTH ******"
			} else {
				query = fmt.Sprintf("AUTH %s ******", username)
			}
			audit.OnQuery(closeCtx, sessionCtx, common.Query{Query: query})
		})
		return username, password, nil
	}
}

// managedUserCredFetchFunc fetches user password on the fly.
func managedUserCredFetchFunc(sessionCtx *common.Session, users common.Users) fetchCredentialsFunc {
	return func(ctx context.Context) (string, string, error) {
		username := sessionCtx.DatabaseUser
		password, err := users.GetPassword(ctx, sessionCtx.Database, username)
		if err != nil {
			return "", "", trace.AccessDenied("failed to get password for %v: %v.",
				username, err)
		}
		return username, password, nil
	}
}

// azureAccessKeyFetchFunc Azure access key for the "default" user.
func azureAccessKeyFetchFunc(sessionCtx *common.Session, auth common.Auth) fetchCredentialsFunc {
	return func(ctx context.Context) (string, string, error) {
		// Retrieve the auth token for Azure Cache for Redis. Use default user.
		password, err := auth.GetAzureCacheForRedisToken(ctx, sessionCtx.Database)
		if err != nil {
			return "", "", trace.AccessDenied("failed to get Azure access key: %v", err)
		}
		// Azure doesn't support ACL yet, so username is left blank.
		return "", password, nil
	}
}

// elasticacheIAMTokenFetchFunc fetches an AWS ElastiCache IAM auth token.
func elasticacheIAMTokenFetchFunc(sessionCtx *common.Session, auth common.Auth) fetchCredentialsFunc {
	return func(ctx context.Context) (string, string, error) {
		// Retrieve the auth token for AWS IAM ElastiCache.
		password, err := auth.GetElastiCacheRedisToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
		if err != nil {
			return "", "", trace.AccessDenied(
				"failed to get AWS ElastiCache IAM auth token for %v: %v",
				sessionCtx.DatabaseUser, err)
		}
		return sessionCtx.DatabaseUser, password, nil
	}
}

// memorydbIAMTokenFetchFunc fetches an AWS MemoryDB IAM auth token.
func memorydbIAMTokenFetchFunc(sessionCtx *common.Session, auth common.Auth) fetchCredentialsFunc {
	return func(ctx context.Context) (string, string, error) {
		password, err := auth.GetMemoryDBToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
		if err != nil {
			return "", "", trace.AccessDenied(
				"failed to get AWS MemoryDB IAM auth token for %v: %v",
				sessionCtx.DatabaseUser, err)
		}
		return sessionCtx.DatabaseUser, password, nil
	}
}

func awsIAMTokenFetchFunc(sessionCtx *common.Session, auth common.Auth) (fetchCredentialsFunc, error) {
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeElastiCache:
		return elasticacheIAMTokenFetchFunc(sessionCtx, auth), nil
	case types.DatabaseTypeMemoryDB:
		return memorydbIAMTokenFetchFunc(sessionCtx, auth), nil
	default:
		// If this happens it means something wrong with our implementation.
		return nil, trace.BadParameter("database type %q not supported for AWS IAM Auth", sessionCtx.Database.GetType())
	}
}

// Process add supports for additional cluster commands. Our Redis implementation passes most commands to
// go-redis `Process()` function which doesn't handel all Cluster commands like for ex. DBSIZE, FLUSHDB, etc.
// This function provides additional processing for those commands enabling more Redis commands in Cluster mode.
// Commands are override by a simple rule:
//   - If command work only on a single slot (one shard) without any modifications and returns a CROSSSLOT error if executed on
//     multiple keys it's send the Redis client as it is, and it's the client responsibility to make sure keys are in a single slot.
//   - If a command returns incorrect result in the Cluster mode (for ex. DBSIZE as it would return only size of one shard not whole cluster)
//     then it's handled by Teleport or blocked if reasonable processing is not possible.
//   - Otherwise a commands is sent to Redis without any modifications.
func (c *clusterClient) Process(ctx context.Context, inCmd redis.Cmder) error {
	cmd, ok := inCmd.(*redis.Cmd)
	if !ok {
		return trace.BadParameter("failed to cast Redis command type: %T", cmd)
	}

	switch cmdName := strings.ToLower(cmd.Name()); cmdName {
	case multiCmd, execCmd, watchCmd, scanCmd, askingCmd, clientCmd, clusterCmd, configCmd, debugCmd,
		infoCmd, latencyCmd, memoryCmd, migrateCmd, moduleCmd, monitorCmd, pfdebugCmd, pfselftestCmd,
		psyncCmd, readonlyCmd, readwriteCmd, replconfCmd, replicaofCmd, roleCmd, shutdownCmd, slaveofCmd,
		slowlogCmd, syncCmd, timeCmd, waitCmd:
		// block commands that return incorrect results in Cluster mode
		return protocol.ErrCmdNotSupported
	case aclCmd:
		// allows "acl whoami" which is a very useful command that works fine
		// in Cluster mode.
		if len(cmd.Args()) == 2 {
			if subcommand, ok := cmd.Args()[1].(string); ok && strings.ToLower(subcommand) == aclWhoami {
				return c.ClusterClient.Process(ctx, cmd)
			}
		}
		// block other "acl" commands.
		return protocol.ErrCmdNotSupported
	case dbsizeCmd:
		// use go-redis dbsize implementation. It returns size of all keys in the whole cluster instead of
		// just currently connected instance.
		res := c.DBSize(ctx)
		cmd.SetVal(res.Val())
		cmd.SetErr(res.Err())

		return nil
	case keysCmd:
		var resultsKeys []string
		var mtx sync.Mutex

		if err := c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			scanCmd := redis.NewStringSliceCmd(ctx, cmd.Args()...)
			err := client.Process(ctx, scanCmd)
			if err != nil {
				return trace.Wrap(err)
			}

			keys, err := scanCmd.Result()
			if err != nil {
				return trace.Wrap(err)
			}

			mtx.Lock()
			resultsKeys = append(resultsKeys, keys...)
			mtx.Unlock()

			return nil
		}); err != nil {
			return trace.Wrap(err)
		}

		cmd.SetVal(resultsKeys)

		return nil
	case mgetCmd:
		if len(cmd.Args()) == 1 {
			return trace.BadParameter("wrong number of arguments for 'mget' command")
		}

		var resultsKeys []interface{}

		keys := cmd.Args()[1:]
		for _, key := range keys {
			k, ok := key.(string)
			if !ok {
				return trace.BadParameter("wrong key type, expected string, got %T", key)
			}

			result := c.Get(ctx, k)
			if errors.Is(result.Err(), redis.Nil) {
				resultsKeys = append(resultsKeys, redis.Nil)
				continue
			}

			if result.Err() != nil {
				cmd.SetErr(result.Err())
				return trace.Wrap(result.Err())
			}

			resultsKeys = append(resultsKeys, result.Val())
		}

		cmd.SetVal(resultsKeys)

		return nil
	case flushallCmd, flushdbCmd:
		var mtx sync.Mutex

		if err := c.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			singleCmd := redis.NewCmd(ctx, cmd.Args()...)
			err := client.Process(ctx, singleCmd)
			if err != nil {
				return trace.Wrap(err)
			}

			mtx.Lock()
			defer mtx.Unlock()

			if cmd.Err() != nil {
				// If other call have already set error do not erase it.
				// It should be returned to the caller.
				return nil
			}

			cmd.SetVal(singleCmd.Val())
			cmd.SetErr(singleCmd.Err())

			return nil
		}); err != nil {
			return trace.Wrap(err)
		}

		return nil
	case scriptCmd:
		return c.handleScriptCmd(ctx, cmd)
	default:
		return c.ClusterClient.Process(ctx, cmd)
	}
}

// handleScriptCmd processes Redis SCRIPT command in Cluster mode.
func (c *clusterClient) handleScriptCmd(ctx context.Context, cmd *redis.Cmd) error {
	cmdArgs := cmd.Args()

	if len(cmdArgs) < 2 {
		return trace.BadParameter("wrong number of arguments for 'script' command")
	}

	args := make([]string, len(cmdArgs))

	for i := range cmdArgs {
		var ok bool
		args[i], ok = cmdArgs[i].(string)
		if !ok {
			return trace.BadParameter("wrong script subcommand type, expected string, got %T", cmdArgs[i])
		}
	}

	switch cmdSubName := strings.ToLower(args[1]); cmdSubName {
	case scriptExistsSubcmd:
		res := c.ScriptExists(ctx, args[2:]...)

		cmd.SetVal(res.Val())
		cmd.SetErr(res.Err())

		return nil
	case scriptLoadSubcmd:
		res := c.ScriptLoad(ctx, args[2])

		cmd.SetVal(res.Val())
		cmd.SetErr(res.Err())

		return nil

	case scriptFLushSubcmd:
		// TODO(jakule): ASYNC is ignored.
		res := c.ScriptFlush(ctx)

		cmd.SetVal(res.Val())
		cmd.SetErr(res.Err())

		return nil
	default:
		// SCRIPT KILL and SCRIPT DEBUG
		return protocol.ErrCmdNotSupported
	}
}
