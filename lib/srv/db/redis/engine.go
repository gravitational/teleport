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
	"net"

	"github.com/go-redis/redis/v8"
	"github.com/gravitational/teleport/lib/srv/db/common"
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

func (e *Engine) SendError(redisErr error) {
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
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	var redisConn redis.UniversalClient

	// TODO:
	// Use system TLS if connecting to AWS.
	// Distinguish between single and cluster instances.
	// Use redis connection string??
	if true {
		redisConn = redis.NewClient(&redis.Options{
			Addr:      sessionCtx.Database.GetURI(),
			TLSConfig: tlsConfig,
		})
	} else {
		redisConn = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:     []string{sessionCtx.Database.GetURI()},
			TLSConfig: tlsConfig,
		})
	}
	defer redisConn.Close()

	e.Log.Debug("created a new Redis client")

	pingResp := redisConn.Ping(context.Background())
	if pingResp.Err() != nil {
		return trace.Wrap(err)
	}

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
