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
	"crypto/tls"
	"io"
	"net"

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
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session, conn net.Conn) error {
	redisConn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		return trace.Wrap(err)
	}
	defer redisConn.Close()

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConn := tls.Client(redisConn, tlsConfig)

	e.Audit.OnSessionStart(ctx, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	go io.Copy(conn, tlsConn)
	err = copyAndLog(e.Log, tlsConn, conn)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func copyAndLog(log logrus.FieldLogger, serverConn io.WriteCloser, clientConn io.ReadCloser) error {
	defer clientConn.Close()
	defer serverConn.Close()

	buf := make([]byte, 1024)
	for {
		n, err := clientConn.Read(buf)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Debugf("redis cmd: %v", string(buf[:n]))

		_, err = serverConn.Write(buf[:n])
		if err != nil {
			return trace.Wrap(err)
		}
	}
}
