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

package redis

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/trace"
)

// List of all commands that Teleport handles in a special way.
const (
	helloCmd        = "hello"
	authCmd         = "auth"
	subscribeCmd    = "subscribe"
	psubscribeCmd   = "psubscribe"
	punsubscribeCmd = "punsubscribe"
	ssubscribeCmd   = "ssubscribe"
	sunsubscribeCmd = "sunsubscribe"
)

// processCmd processes commands received from connected client. Most commands are just passed to Redis instance,
// but some require special actions:
//  * Redis 7.0+ commands are rejected as at the moment of writing Redis 7.0 hasn't been released and go-redis doesn't support it.
//  * RESP3 commands are rejected as Teleport currently doesn't support this version of protocol.
//  * Subscribe related commands created a new DB connection as they change Redis request-response model to Pub/Sub.
func (e *Engine) processCmd(ctx context.Context, redisClient redis.UniversalClient, cmd *redis.Cmd) error {
	switch strings.ToLower(cmd.Name()) {
	case helloCmd:
		return trace.NotImplemented("RESP3 is not supported")
	case authCmd:
		if err := e.processAuth(ctx, redisClient, cmd); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case subscribeCmd:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: cmd.String()})
		if err := e.subscribeCmd(ctx, redisClient.Subscribe, cmd); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case psubscribeCmd:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: cmd.String()})
		if err := e.subscribeCmd(ctx, redisClient.PSubscribe, cmd); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case punsubscribeCmd:
		// TODO(jakub): go-redis doesn't expose any API for this command. Investigate alternative options.
		return trace.NotImplemented("PUNSUBSCRIBE is not supported by Teleport")
	case ssubscribeCmd, sunsubscribeCmd:
		// go-redis doesn't support Redis 7+ commands yet.
		return trace.NotImplemented("Redis 7.0+ is not yet supported")
	default:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: cmd.String()})

		// Here the command is sent to the DB.
		return redisClient.Process(ctx, cmd)
	}
}

type redisSubscribeFn = func(ctx context.Context, channels ...string) *redis.PubSub

// subscribeCmd handles subscription in Redis. A new connection is created and maintained to read published messages
// from Redis and send them back to the client.
func (e *Engine) subscribeCmd(ctx context.Context, subscribeFn redisSubscribeFn, cmd *redis.Cmd) error {
	if len(cmd.Args()) < 2 {
		return redis.RedisError("invalid command")
	}

	args := make([]string, 0, len(cmd.Args()))
	for _, a := range cmd.Args()[1:] {
		args = append(args, a.(string))
	}
	pubSub := subscribeFn(ctx, args...)
	defer func() {
		if err := pubSub.Close(); err != nil {
			e.Log.Errorf("failed to close Redis Pub/Sub connection: %v", err)
		}
	}()

	return e.processPubSub(ctx, pubSub)
}

// processPubSub reads messages from Redis and forward them to connected client.
func (e *Engine) processPubSub(ctx context.Context, pubSub *redis.PubSub) error {
	for {
		msg, err := pubSub.Receive(ctx)
		if err != nil {
			return err
		}

		switch msg := msg.(type) {
		case *redis.Subscription:
			if err := e.sendToClient([]interface{}{msg.Kind, msg.Channel, msg.Count}); err != nil {
				return err
			}
		case *redis.Pong:
			if err := e.sendToClient([]interface{}{msg.Payload}); err != nil {
				return err
			}
		case *redis.Message:
			var payloadResp interface{}
			if len(msg.PayloadSlice) > 0 {
				payloadResp = []interface{}{"message", msg.Channel, msg.PayloadSlice}
			} else {
				payloadResp = []interface{}{"message", msg.Channel, msg.Payload}
			}

			if err := e.sendToClient(payloadResp); err != nil {
				return err
			}
		default:
			err := fmt.Errorf("redis: unknown message: %T", msg)
			return err
		}
	}
}

// processAuth runs RBAC check on Redis AUTH command if command contains username. Command containing only password
// is passed to Redis. Commands with incorrect number of arguments are rejected and an error is returned.
func (e *Engine) processAuth(ctx context.Context, redisClient redis.UniversalClient, cmd *redis.Cmd) error {
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
			services.AccessMFAParams{Verified: true},
			role.DatabaseRoleMatchers(
				defaults.ProtocolRedis,
				e.sessionCtx.DatabaseUser,
				e.sessionCtx.DatabaseName,
			)...)
		if err != nil {
			return trace.Wrap(err)
		}

		return redisClient.Process(ctx, cmd)
	case 3:
		// Redis 6 version that contains username and password. Check the username against our RBAC before sending to Redis.
		// ex. AUTH bob my-secret-password
		dbUser, ok := cmd.Args()[1].(string)
		if !ok {
			return trace.BadParameter("username has a wrong type, expected string")
		}

		if dbUser != e.sessionCtx.DatabaseUser {
			return trace.AccessDenied("failed to authenticate as %s user. "+
				"Please provide a correct db username when connecting to Redis", dbUser)
		}

		err := e.sessionCtx.Checker.CheckAccess(e.sessionCtx.Database,
			services.AccessMFAParams{Verified: true},
			role.DatabaseRoleMatchers(
				defaults.ProtocolRedis,
				e.sessionCtx.DatabaseUser,
				e.sessionCtx.DatabaseName,
			)...)
		if err != nil {
			return trace.Wrap(err)
		}

		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{Query: fmt.Sprintf("AUTH %s ****", dbUser)})

		return redisClient.Process(ctx, cmd)
	default:
		// Redis returns "syntax error" if AUTH has more than 2 arguments.
		return trace.BadParameter("syntax error")
	}
}
