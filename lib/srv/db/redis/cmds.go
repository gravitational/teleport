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
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/redis/protocol"
)

// List of commands that Teleport handles in a special way by Redis standalone and cluster.
const (
	helloCmd      = "hello"
	authCmd       = "auth"
	subscribeCmd  = "subscribe"
	psubscribeCmd = "psubscribe"
	// TODO(jakub): go-redis doesn't expose any API for this command. Investigate alternative options.
	punsubscribeCmd = "punsubscribe"
	// go-redis doesn't support Redis 7+ commands yet.
	ssubscribeCmd   = "ssubscribe"
	sunsubscribeCmd = "sunsubscribe"
)

// processCmd processes commands received from connected client. Most commands are just passed to Redis instance,
// but some require special actions:
//   - Redis 7.0+ commands are rejected as at the moment of writing Redis 7.0 hasn't been released and go-redis doesn't support it.
//   - RESP3 commands are rejected as Teleport/go-redis currently doesn't support this version of protocol.
//   - Subscribe related commands created a new DB connection as they change Redis request-response model to Pub/Sub.
func (e *Engine) processCmd(ctx context.Context, cmd *redis.Cmd) error {
	switch strings.ToLower(cmd.Name()) {
	case helloCmd:
		// HELLO command is still not supported yet by Teleport. However, some
		// Redis clients (e.g. go-redis) may explicitly look for the original
		// Redis unknown command error so it can fallback to RESP2.
		return protocol.MakeUnknownCommandErrorForCmd(cmd)
	case punsubscribeCmd, ssubscribeCmd, sunsubscribeCmd:
		return protocol.ErrCmdNotSupported
	case authCmd:
		return e.processAuth(ctx, cmd)
	case subscribeCmd:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: cmd.String()})
		return e.subscribeCmd(ctx, e.redisClient.Subscribe, cmd)
	case psubscribeCmd:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: cmd.String()})
		return e.subscribeCmd(ctx, e.redisClient.PSubscribe, cmd)
	default:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: cmd.String()})

		// Here the command is sent to the DB.
		return e.redisClient.Process(ctx, cmd)
	}
}

type redisSubscribeFn = func(ctx context.Context, channels ...string) *redis.PubSub

// subscribeCmd handles subscription in Redis. A new connection is created and maintained to read published messages
// from Redis and send them back to the client.
func (e *Engine) subscribeCmd(ctx context.Context, subscribeFn redisSubscribeFn, cmd *redis.Cmd) error {
	if len(cmd.Args()) < 2 {
		return trace.BadParameter("invalid command")
	}

	args := make([]string, 0, len(cmd.Args()))
	for _, arg := range cmd.Args()[1:] {
		argStr, ok := arg.(string)
		if !ok {
			return trace.BadParameter("wrong argument %s type, expected string", cmd.Name())
		}
		args = append(args, argStr)
	}
	pubSub := subscribeFn(ctx, args...)
	defer func() {
		if err := pubSub.Close(); err != nil {
			e.Log.ErrorContext(ctx, "Failed to close Redis Pub/Sub connection.", "error", err)
		}
	}()

	return e.processPubSub(ctx, pubSub)
}

// processPubSub reads messages from Redis and forward them to connected client.
func (e *Engine) processPubSub(ctx context.Context, pubSub *redis.PubSub) error {
	for {
		msg, err := pubSub.Receive(ctx)
		if err != nil {
			if errors.Is(err, redis.ErrClosed) {
				// connection has been closed, return no error.
				return nil
			}
			return trace.Wrap(err)
		}

		switch msg := msg.(type) {
		case *redis.Subscription:
			if err := e.sendToClient([]any{msg.Kind, msg.Channel, msg.Count}); err != nil {
				return trace.Wrap(err)
			}
		case *redis.Pong:
			if err := e.sendToClient([]any{msg.Payload}); err != nil {
				return trace.Wrap(err)
			}
		case *redis.Message:
			var payloadResp []any
			if msg.Pattern != "" {
				// pattern is only set when the subscription type is pmessage
				payloadResp = []any{"pmessage", msg.Pattern, msg.Channel}
			} else {
				payloadResp = []any{"message", msg.Channel}
			}

			if len(msg.PayloadSlice) > 0 {
				payloadResp = append(payloadResp, msg.PayloadSlice)
			} else {
				payloadResp = append(payloadResp, msg.Payload)
			}

			if err := e.sendToClient(payloadResp); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("redis: unknown message: %T", msg)
		}
	}
}

// processAuth runs RBAC check on Redis AUTH command if command contains username. Command containing only password
// is passed to Redis. Commands with incorrect number of arguments are rejected and an error is returned.
func (e *Engine) processAuth(ctx context.Context, cmd *redis.Cmd) error {
	// AUTH command may contain only password or login and password. Depends on the version we need to make sure
	// that the user has permission to connect as the provided db user.
	// ref: https://redis.io/commands/auth
	switch len(cmd.Args()) {
	case 1:
		// Passed AUTH command without any arguments. Mimic Redis error response.
		return trace.BadParameter("wrong number of arguments for 'auth' command")
	case 2:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: "AUTH ****"})
		// Old Redis command. Password is the only argument here. Pass to Redis to validate.
		// ex. AUTH my-secret-password

		// Redis sets "default" as a default username. Here we need to check if the "implicit" username
		// matches the one provided as teleport db-user.
		// ref: https://redis.io/commands/auth
		if e.sessionCtx.DatabaseUser != defaults.DefaultRedisUsername {
			return trace.AccessDenied("failed to authenticate as the default user. " +
				"Please provide the db username when connecting to Redis")
		}

		err := e.sessionCtx.Checker.CheckAccess(e.sessionCtx.Database,
			services.AccessState{MFAVerified: true},
			role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
				Database:     e.sessionCtx.Database,
				DatabaseUser: e.sessionCtx.DatabaseUser,
				DatabaseName: e.sessionCtx.DatabaseName,
			})...)
		if err != nil {
			return trace.Wrap(err)
		}

		password, ok := cmd.Args()[1].(string)
		if !ok {
			return trace.BadParameter("password has a wrong type, expected string got %T", cmd.Args()[1])
		}

		// Pass empty username to login using AUTH <password> command.
		e.redisClient, err = e.reconnect("", password)
		if err != nil {
			return trace.Wrap(err)
		}

		return e.redisClient.Process(ctx, cmd)
	case 3:
		// Redis 6 version that contains username and password. Check the username against our RBAC before sending to Redis.
		// ex. AUTH bob my-secret-password
		dbUser, ok := cmd.Args()[1].(string)
		if !ok {
			return trace.BadParameter("username has a wrong type, expected string got %T", cmd.Args()[1])
		}

		// For Teleport managed users, bypass the passwords sent here.
		if slices.Contains(e.sessionCtx.Database.GetManagedUsers(), e.sessionCtx.DatabaseUser) {
			return trace.Wrap(e.sendToClient([]string{
				"OK",
				fmt.Sprintf("Please note that AUTH commands are ignored for Teleport managed user '%s'.", e.sessionCtx.DatabaseUser),
				"Teleport service automatically authenticates managed users with the Redis server.",
			}))
		}

		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: fmt.Sprintf("AUTH %s ****", dbUser)})

		if dbUser != e.sessionCtx.DatabaseUser {
			return trace.AccessDenied("failed to authenticate as %s user. "+
				"Please provide a correct db username when connecting to Redis", dbUser)
		}

		err := e.sessionCtx.Checker.CheckAccess(e.sessionCtx.Database,
			services.AccessState{MFAVerified: true},
			role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
				Database:     e.sessionCtx.Database,
				DatabaseUser: e.sessionCtx.DatabaseUser,
				DatabaseName: e.sessionCtx.DatabaseName,
			})...)
		if err != nil {
			return trace.Wrap(err)
		}

		password, ok := cmd.Args()[2].(string)
		if !ok {
			return trace.BadParameter("password has a wrong type, expected string got %T", cmd.Args()[2])
		}

		// reconnect with new username and password. go-redis manages those internally and sends AUTH commands
		// at the beginning of every new connection.
		e.redisClient, err = e.reconnect(dbUser, password)
		if err != nil {
			return trace.Wrap(err)
		}

		return e.redisClient.Process(ctx, cmd)
	default:
		// Redis returns "syntax error" if AUTH has more than 2 arguments.
		return trace.BadParameter("syntax error")
	}
}
