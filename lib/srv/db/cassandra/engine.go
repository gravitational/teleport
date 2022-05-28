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
	"fmt"
	"io"
	"net"

	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/datastax/go-cassandra-native-protocol/segment"
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
	if utils.IsOKNetworkError(err) || err == nil {
		return
	}

	e.Log.Errorf("cassandra connection error: %v", err)
}

func (e *Engine) sendClientError(protoVer primitive.ProtocolVersion, streamID int16, err error) {
	var errFrame *frame.Frame

	if trace.IsAccessDenied(err) {
		errFrame = frame.NewFrame(protoVer, streamID, &message.AuthenticationError{ErrorMessage: err.Error()})
	} else {
		errFrame = frame.NewFrame(protoVer, streamID, &message.ServerError{ErrorMessage: err.Error()})
	}

	codec := frame.NewRawCodec()
	if err := codec.EncodeFrame(errFrame, e.clientConn); err != nil {
		e.Log.Errorf("failed to send error message to the client: %v", err)
	}
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
		e.sendAuthError(err)
		return trace.Wrap(err)
	}

	codec := frame.NewRawCodec()
	segmentCodec := segment.NewCodec()
	isV5 := false

	e.Log.Info("Accepted new connection")

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	bconn := newInMemoryReader(e.clientConn)

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	srvConn, err := tls.Dial("tcp", sessionCtx.Database.GetURI(), tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	go io.Copy(e.clientConn, srvConn)

	for {
		var rawFrame *frame.RawFrame

		if isV5 {
			seg, err := segmentCodec.DecodeSegment(bconn)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return trace.Wrap(err, "failed to decode frame")
			}

			if !seg.Header.IsSelfContained {
				return trace.NotImplemented("not self contained frames are not implemented")
			}

			payloadReader := bytes.NewReader(seg.Payload.UncompressedData)

			rawFrame, err = codec.DecodeRawFrame(payloadReader)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return trace.Wrap(err, "failed to decode frame")
			}
		} else {
			rawFrame, err = codec.DecodeRawFrame(bconn)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return trace.Wrap(err, "failed to decode frame")
			}
		}

		//f, err := rawFrame.Dump()
		//if err != nil {
		//	return trace.Wrap(err, "failed to dump frame")
		//}
		//
		//e.Log.Infof("frame: %v", f)
		//e.Log.Infof("OpCode: %+v", rawFrame.Header.OpCode)

		switch rawFrame.Header.OpCode {
		case primitive.OpCodeStartup:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode auth response")
			}

			if startup, ok := body.Body.Message.(*message.Startup); ok {
				if body.Header.Version == primitive.ProtocolVersion5 {
					// TODO(jakule): we should also check the supported version returned by the DB, not only client.
					isV5 = true
				}
				compression := startup.GetCompression()
				e.Log.Infof("compression: %v", compression)
				codec = frame.NewRawCodecWithCompression(client.NewBodyCompressor(compression))
				segmentCodec = segment.NewCodecWithCompression(client.NewPayloadCompressor(compression))
			}
		case primitive.OpCodeAuthResponse:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode auth response")
			}

			if authResp, ok := body.Body.Message.(*message.AuthResponse); ok {
				// auth token contains username and password split by \0 character
				// ex: \0username\0password
				data := bytes.Split(authResp.Token, []byte{0})
				if len(data) != 3 {
					return trace.BadParameter("failed to extract username from the auth package.")
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
					Query: query.String(),
				})
			}
		case primitive.OpCodePrepare:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode prepare")
			}

			if prepare, ok := body.Body.Message.(*message.Prepare); ok {
				e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
					Query: prepare.String(),
				})
			}
		case primitive.OpCodeExecute:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode prepare")
			}

			if execute, ok := body.Body.Message.(*message.Execute); ok {
				e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
					Query: execute.String(),
				})
			}
		case primitive.OpCodeBatch:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode batch")
			}

			if batch, ok := body.Body.Message.(*message.Batch); ok {
				queries := make([]string, 0, len(batch.Children))
				for _, child := range batch.Children {
					queries = append(queries, fmt.Sprintf("%+v, values: %v", child.QueryOrId, child.Values))
				}

				e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
					Query: fmt.Sprintf("begin batch %s batch apply", queries),
					Parameters: []string{
						"consistency", batch.Consistency.String(),
						"keyspace", batch.Keyspace,
						"batch", batch.Type.String(),
					},
				})
			}
		case primitive.OpCodeRegister:
			body, err := codec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				return trace.Wrap(err, "failed to decode register")
			}

			if register, ok := body.Body.Message.(*message.Register); ok {
				e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
					Query: register.String(),
				})
			}
		}

		_, err = srvConn.Write(bconn.Bytes())
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

func (e *Engine) sendAuthError(accessErr error) {
	codec := frame.NewRawCodec()

	for {
		rawFrame, err := codec.DecodeRawFrame(e.clientConn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				e.Log.Infof("failed to decode frame: %v", err)
				return
			}
			e.Log.WithError(err).Error("failed to send encode a frame")
		}

		switch rawFrame.Header.OpCode {
		case primitive.OpCodeStartup:
			startupFrame := frame.NewFrame(rawFrame.Header.Version, rawFrame.Header.StreamId,
				&message.Authenticate{Authenticator: "org.apache.cassandra.auth.PasswordAuthenticator"})

			if err := codec.EncodeFrame(startupFrame, e.clientConn); err != nil {
				e.Log.WithError(err).Error("failed to send startup frame")
				return
			}
		case primitive.OpCodeAuthResponse:
			e.sendClientError(rawFrame.Header.Version, rawFrame.Header.StreamId, accessErr)
			return
		case primitive.OpCodeOptions:
			supportedFrame := frame.NewFrame(rawFrame.Header.Version, rawFrame.Header.StreamId, &message.Supported{})
			if err := codec.EncodeFrame(supportedFrame, e.clientConn); err != nil {
				e.Log.WithError(err).Error("failed to send startup frame")
				return
			}
		}
	}
}

type inMemoryReader struct {
	buff []byte
	r    io.Reader
}

func newInMemoryReader(reader io.Reader) *inMemoryReader {
	return &inMemoryReader{
		buff: make([]byte, 0),
		r:    reader,
	}
}

func (b *inMemoryReader) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.buff = append(b.buff, p...)
	return n, err
}

func (b *inMemoryReader) Bytes() []byte {
	return b.buff
}

func (b *inMemoryReader) Reset() {
	b.buff = b.buff[:0]
}
