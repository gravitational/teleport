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
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func init() {
	common.RegisterEngine(newEngine, defaults.ProtocolCassandra)
}

// newEngine create new Cassandra engine.
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
	err := e.authorizeConnection(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

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

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

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
		//e.Log.Infof("OpCode: %+v", rawFrame.Header.OpCode)

		switch rawFrame.Header.OpCode {
		case primitive.OpCodeAuthResponse:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode auth response")
			}

			if authResp, ok := body.Body.Message.(*message.AuthResponse); ok {
				data := bytes.Split(authResp.Token, []byte{0})
				if len(data) != 3 {
					return trace.BadParameter("failed to extract username from auth package.")
				}
				username := string(data[1])

				e.Log.Infof("auth response: %s, %s", username, string(data[2]))

				if e.sessionCtx.DatabaseUser != username {
					return trace.AccessDenied("user %s is not authorized to access the database", username)
				}
			}
		case primitive.OpCodeQuery:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode query")
			}

			if query, ok := body.Body.Message.(*message.Query); ok {
				e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
					Query:      query.Query,
					Parameters: []string{"keyspace", query.Options.Keyspace}, //TODO(jakule): What is the correct format?
				})
			}
		}

		_, err = srvConn.Write(bconn.Buff())
		if err != nil {
			return trace.Wrap(err, "failed to write frame to cassandra: %v", err)
		}

		bconn.Reset()
	}
}

// authorizeConnection does authorization check for Cassandra connection about
// to be established.
func (e *Engine) authorizeConnection(ctx context.Context) error {
	ap, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	mfaParams := services.AccessMFAParams{
		Verified:       e.sessionCtx.Identity.MFAVerified != "",
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}

	dbRoleMatchers := role.DatabaseRoleMatchers(
		e.sessionCtx.Database.GetProtocol(),
		e.sessionCtx.DatabaseUser,
		e.sessionCtx.DatabaseName,
	)
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		mfaParams,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
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
