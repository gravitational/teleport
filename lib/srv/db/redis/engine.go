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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/go-redis/redis/v8"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Engine implements common.Engine.
type Engine struct {
	// Auth handles database access authentication.
	Auth common.Auth
	// Audit emits database access audit events.
	Audit common.Audit
	// Context is the database server close context.
	Context context.Context
	// Clock is the clock interface.
	Clock clockwork.Clock
	// Log is used for logging.
	Log logrus.FieldLogger
	// proxyConn is a client connection.
	proxyConn net.Conn
}

func (e *Engine) InitializeConnection(clientConn net.Conn, _ *common.Session) error {
	e.proxyConn = clientConn

	return nil
}

// authorizeConnection does authorization check for MongoDB connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context, sessionCtx *common.Session) error {
	ap, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	mfaParams := services.AccessMFAParams{
		Verified:       sessionCtx.Identity.MFAVerified != "",
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}

	dbRoleMatchers := role.DatabaseRoleMatchers(
		sessionCtx.Database.GetProtocol(),
		sessionCtx.DatabaseUser,
		sessionCtx.DatabaseName,
	)
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		mfaParams,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) SendError(redisErr error) {
	if redisErr == nil || utils.IsOKNetworkError(redisErr) {
		return
	}

	e.Log.Debugf("sending error to Redis client: %v", redisErr)

	buf := &bytes.Buffer{}
	wr := redis.NewWriter(buf)

	if err := writeCmd(wr, redisErr); err != nil {
		e.Log.Errorf("Failed to convert error to a message: %v", err)
		return
	}

	if _, err := e.proxyConn.Write(buf.Bytes()); err != nil {
		e.Log.Errorf("Failed to send message to the client: %v", err)
		return
	}
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	// Check that the user has access to the database.
	err := e.authorizeConnection(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err, "error authorizing database access")
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	connectionOptions, err := ParseRedisURI(sessionCtx.Database.GetURI())
	if err != nil {
		return trace.BadParameter("Redis connection string is incorrect: %v", err)
	}

	var (
		redisConn      redis.UniversalClient
		connectionAddr = fmt.Sprintf("%s:%s", connectionOptions.address, connectionOptions.port)
	)

	// TODO(jakub): Use system CA bundle if connecting to AWS.
	if connectionOptions.cluster {
		redisConn = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:     []string{connectionAddr},
			TLSConfig: tlsConfig,
		})
	} else {
		redisConn = redis.NewClient(&redis.Options{
			Addr:      connectionAddr,
			TLSConfig: tlsConfig,
		})
	}
	defer redisConn.Close()

	e.Log.Debug("created a new Redis client, sending ping")

	pingResp := redisConn.Ping(context.Background())
	if pingResp.Err() != nil {
		return trace.Wrap(pingResp.Err())
	}

	e.Log.Debug("Redis server responded to ping message")

	if err := e.process(ctx, e.proxyConn, redisConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *Engine) process(ctx context.Context, clientConn net.Conn, redisClient redis.UniversalClient) error {
	clientReader := redis.NewReader(clientConn)
	buf := &bytes.Buffer{}
	wr := redis.NewWriter(buf)
	var redisErr redis.Error

	for {
		cmd := &redis.Cmd{}
		if err := cmd.ReadReply(clientReader); err != nil {
			return trace.Wrap(err)
		}

		val, ok := cmd.Val().([]interface{})
		if !ok {
			return trace.BadParameter("failed to cast Redis value to a slice")
		}

		nCmd := redis.NewCmd(ctx, val...)

		e.Log.Debugf("redis cmd: %v", nCmd.Name())

		err := redisClient.Process(ctx, nCmd)

		var vals interface{}

		if errors.As(err, &redisErr) {
			vals = err
		} else if err != nil {
			// Send the error to the client as connection will be terminated after return.
			e.SendError(err)

			return trace.Wrap(err)
		} else {
			vals, err = nCmd.Result()
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if err := writeCmd(wr, vals); err != nil {
			return trace.Wrap(err)
		}

		if _, err := clientConn.Write(buf.Bytes()); err != nil {
			return trace.Wrap(err)
		}

		buf.Reset()
	}
}

func writeCmd(wr *redis.Writer, vals interface{}) error {
	switch val := vals.(type) {
	case error:
		if err := wr.WriteByte('-'); err != nil {
			return trace.Wrap(err)
		}

		if _, err := wr.WriteString(val.Error()); err != nil {
			return trace.Wrap(err)
		}

		if _, err := wr.Write([]byte("\r\n")); err != nil {
			return trace.Wrap(err)
		}

	case []interface{}:
		if err := wr.WriteByte(redis.ArrayReply); err != nil {
			return trace.Wrap(err)
		}
		n := len(val)
		if err := wr.WriteLen(n); err != nil {
			return trace.Wrap(err)
		}

		for _, v0 := range val {
			if err := writeCmd(wr, v0); err != nil {
				return trace.Wrap(err)
			}
		}
	case interface{}:
		err := wr.WriteArg(val)
		if err != nil {
			return err
		}
	}

	return nil
}
