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

package cassandra

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func init() {
	common.RegisterEngine(newEngine, defaults.ProtocolCassandra)
}

// newEngine create new Redis engine.
func newEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
}

func (e *Engine) SendError(err error) {
	if utils.IsOKNetworkError(err) {
		return
	}

	e.Log.Errorf("cassandra connection error: %v", err)
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx

	return nil
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	codec := frame.NewRawCodec()

	e.Log.Info("Accepted new connection")

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	bconn := &buffConn{
		buff: make([]byte, 0),
		r:    e.clientConn,
	}

	srvConn, err := tls.Dial("tcp", sessionCtx.Database.GetURI(), tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	go io.Copy(e.clientConn, srvConn)

	for {
		rawFrame, err := codec.DecodeRawFrame(bconn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err, "failed to decode frame")
		}

		f, err := rawFrame.Dump()
		if err != nil {
			return trace.Wrap(err, "failed to dump frame")
		}

		e.Log.Infof("frame: %v", f)

		switch rawFrame.Header.OpCode {
		case primitive.OpCodeQuery:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode query")
			}

			if query, ok := body.Body.Message.(*message.Query); ok {
				e.Log.Infof("query: %s", query.Query)
			}
		}

		_, err = srvConn.Write(bconn.Buff())
		if err != nil {
			return trace.Wrap(err, "failed to write frame to cassandra: %v", err)
		}

		bconn.Reset()
	}
}

type buffConn struct {
	buff []byte
	r    io.Reader
}

func (b *buffConn) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.buff = append(b.buff, p...)
	return n, err
}

func (b *buffConn) Buff() []byte {
	return b.buff
}

func (b *buffConn) Reset() {
	b.buff = b.buff[:0]
}
