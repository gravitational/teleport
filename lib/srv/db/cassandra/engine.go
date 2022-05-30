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
	"sync"

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
		mtx:          &sync.Mutex{},
		frameCodec:   frame.NewRawCodec(),
		segmentCodec: segment.NewCodec(),
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

	mtx *sync.Mutex

	frameCodec frame.RawCodec

	segmentCodec segment.Codec

	v5Layout bool
}

func (e *Engine) SendError(err error) {
	if utils.IsOKNetworkError(err) || err == nil {
		return
	}

	// TODO implement
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

	e.Log.Info("Accepted new connection")

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	serverConn, err := tls.Dial("tcp", sessionCtx.Database.GetURI(), tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := serverConn.Close(); err != nil {
			e.Log.WithError(err).Error("failed to close server connection")
		}
	}()

	go func() {
		err := e.handleServerConnection(serverConn)
		if err != nil {
			e.Log.WithError(err).Error("failed to proxy server data")
		}
	}()

	return e.handleClientConnection(serverConn)
}

func (e *Engine) handleClientConnection(serverConn *tls.Conn) error {
	memBuf := newMemoryBuffer(e.clientConn)
	previousSegment := bytes.Buffer{}
	expectedSegmentSize := 0

	for {
		var payloadReader io.Reader
		if e.v5Layout {
			e.Log.Infof("using modern layout")
			seg, err := e.segmentCodec.DecodeSegment(memBuf)
			if err != nil {
				if errors.Is(err, io.EOF) || utils.IsOKNetworkError(err) {
					return nil
				}
				return trace.Wrap(err, "failed to decode frame")
			}

			e.Log.Infof("is self contained: %t, crc: %d", seg.Header.IsSelfContained, seg.Header.Crc24)
			if !seg.Header.IsSelfContained {
				e.Log.Infof("reading not self contained segment: prevSize %d, accSize %d", previousSegment.Len(), expectedSegmentSize)
				if expectedSegmentSize == 0 {
					frameHeader, err := e.frameCodec.DecodeHeader(bytes.NewReader(seg.Payload.UncompressedData))
					if err != nil {
						return trace.Wrap(err)
					}
					expectedSegmentSize = int(frameHeader.BodyLength + 9)
				}
				previousSegment.Write(seg.Payload.UncompressedData)

				e.Log.Infof("second log: prevSize %d, accSize %d, crc: %x", previousSegment.Len(), expectedSegmentSize, seg.Header.Crc24)
				if expectedSegmentSize == previousSegment.Len() {
					payloadReader = &previousSegment
					expectedSegmentSize = 0
				} else {
					continue
				}
			} else {
				payloadReader = bytes.NewReader(seg.Payload.UncompressedData)
			}
		} else {
			e.Log.Infof("using old layout")
			payloadReader = memBuf
		}

		if err := e.processFrame(payloadReader); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err, "failed to process frame")
		}

		if _, err := serverConn.Write(memBuf.Bytes()); err != nil {
			return trace.Wrap(err, "failed to write frame to cassandra: %v", err)
		}

		memBuf.Reset()
		previousSegment.Reset()
	}
}

func (e *Engine) handleServerConnection(serverConn *tls.Conn) error {
	r := io.TeeReader(serverConn, e.clientConn)

	for {
		rawFrame, err := e.frameCodec.DecodeRawFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err)
		}

		e.Log.Infof("server opcode: %v", rawFrame.Header.OpCode)
		if rawFrame.Header.OpCode == primitive.OpCodeSupported {
			body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
			if err != nil {
				e.Log.Errorf("failed to convert a frame: %v", err)
				return trace.Wrap(err)
			}

			if startup, ok := body.Body.Message.(*message.Supported); ok {
				e.Log.Infof("supported response: %v", startup)
				e.v5Layout = rawFrame.Header.Version == primitive.ProtocolVersion5
				break
			}
		}
	}

	if _, err := io.Copy(e.clientConn, serverConn); err != nil && !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (e *Engine) processFrame(conn io.Reader) error {
	rawFrame, err := e.frameCodec.DecodeRawFrame(conn)
	if err != nil {
		return trace.Wrap(err, "failed to decode frame")
	}

	switch rawFrame.Header.OpCode {
	case primitive.OpCodeStartup:
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
		if err != nil {
			return trace.Wrap(err, "failed to decode auth response")
		}

		if startup, ok := body.Body.Message.(*message.Startup); ok {
			//if body.Header.Version == primitive.ProtocolVersion5 {
			//	e.Log.Infof("switching to new layout")
			//	// TODO(jakule): we should also check the supported version returned by the DB, not only client.
			//	e.v5Layout = true
			//}
			compression := startup.GetCompression()
			e.Log.Infof("compression: %v", compression)
			e.frameCodec = frame.NewRawCodecWithCompression(client.NewBodyCompressor(compression))
			e.segmentCodec = segment.NewCodecWithCompression(client.NewPayloadCompressor(compression))
		}
	case primitive.OpCodeAuthResponse:
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
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
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
		if err != nil {
			return trace.Wrap(err, "failed to decode query")
		}

		if query, ok := body.Body.Message.(*message.Query); ok {
			queryStr := query.String()
			if len(queryStr) > 100 {
				queryStr = queryStr[:100]
			}
			e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
				Query: queryStr,
			})
		}
	case primitive.OpCodePrepare:
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
		if err != nil {
			return trace.Wrap(err, "failed to decode prepare")
		}

		if prepare, ok := body.Body.Message.(*message.Prepare); ok {
			e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
				Query: prepare.String(),
			})
		}
	case primitive.OpCodeExecute:
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
		if err != nil {
			return trace.Wrap(err, "failed to decode prepare")
		}

		if execute, ok := body.Body.Message.(*message.Execute); ok {
			e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
				Query: execute.String(),
			})
		}
	case primitive.OpCodeBatch:
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
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
		body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
		if err != nil {
			return trace.Wrap(err, "failed to decode register")
		}

		if register, ok := body.Body.Message.(*message.Register); ok {
			e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
				Query: register.String(),
			})
		}
	}

	return trace.Wrap(err)
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
			// Downgrade client if needed to simplify communication.
			//minSupportedProtocol := primitive.ProtocolVersion3
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
