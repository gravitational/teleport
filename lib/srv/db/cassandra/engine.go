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
	"strings"
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

	for {
		payloadReader, err := e.maybeReadFromClient(memBuf)
		if err != nil {
			if errors.Is(err, io.EOF) || utils.IsOKNetworkError(err) {
				return nil
			}
			return trace.Wrap(err, "failed to read frame")
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

		// Reset buffered frames.
		memBuf.Reset()
	}
}

func (e *Engine) maybeReadFromClient(memBuf *memoryBuffer) (io.Reader, error) {
	if e.v5Layout {
		return e.readV5Layout(memBuf)
	}

	// protocol v4 and earlier
	return memBuf, nil
}

func (e *Engine) readV5Layout(memBuf io.Reader) (io.Reader, error) {
	previousSegment := bytes.Buffer{}
	expectedSegmentSize := 0

	for {
		seg, err := e.segmentCodec.DecodeSegment(memBuf)
		if err != nil {
			return nil, trace.Wrap(err, "failed to decode frame")
		}

		// If the frame is self-contained return it.
		if seg.Header.IsSelfContained {
			return bytes.NewReader(seg.Payload.UncompressedData), nil
		}

		// Otherwise read the frame size and keep reading until we read all data.
		// Segments are always delivered in order.
		if expectedSegmentSize == 0 {
			frameHeader, err := e.frameCodec.DecodeHeader(bytes.NewReader(seg.Payload.UncompressedData))
			if err != nil {
				return nil, trace.Wrap(err)
			}
			expectedSegmentSize = int(primitive.FrameHeaderLengthV3AndHigher + frameHeader.BodyLength)
		}
		// Append another segment
		previousSegment.Write(seg.Payload.UncompressedData)

		// Return the frame after reading all segments.
		if expectedSegmentSize == previousSegment.Len() {
			return &previousSegment, nil
		}
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

	body, err := e.frameCodec.ConvertFromRawFrame(rawFrame)
	if err != nil {
		return trace.Wrap(err, "failed to decode auth response")
	}

	switch msg := body.Body.Message.(type) {
	case *message.Options:
		// NOOP - this message is used as a ping/heartbeat.
	case *message.Startup:
		// TODO(jakule): compression for some reason it returned lowercase :(
		compression := primitive.Compression(strings.ToUpper(string(msg.GetCompression())))
		e.Log.Infof("compression: %v", compression)
		e.frameCodec = frame.NewRawCodecWithCompression(client.NewBodyCompressor(compression))
		e.segmentCodec = segment.NewCodecWithCompression(client.NewPayloadCompressor(compression))
	case *message.AuthResponse:
		// auth token contains username and password split by \0 character
		// ex: \0username\0password
		data := bytes.Split(msg.Token, []byte{0})
		if len(data) != 3 {
			return trace.BadParameter("failed to extract username from the auth package.")
		}
		username := string(data[1])

		e.Log.Infof("auth response: %s, %s", username, string(data[2]))

		if e.sessionCtx.DatabaseUser != username {
			return trace.AccessDenied("user %s is not authorized to access the database", username)
		}
	case *message.Query:
		queryStr := msg.String()
		// TODO(jakule): what to do if query exceeds 65k?
		if len(queryStr) > 100 {
			queryStr = queryStr[:100]
		}
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
			Query: queryStr,
		})
	case *message.Prepare:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
			Query: msg.String(),
		})
	case *message.Execute:
		// TODO(jakule): Execute log is not probably very useful as it contains only
		// query ID returned by PREPARE command.
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
			Query: msg.String(),
		})
	case *message.Batch:
		queries := make([]string, 0, len(msg.Children))
		for _, child := range msg.Children {
			queries = append(queries, fmt.Sprintf("%+v, values: %v", child.QueryOrId, child.Values))
		}

		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
			Query: fmt.Sprintf("begin batch %s batch apply", queries),
			Parameters: []string{
				"consistency", msg.Consistency.String(),
				"keyspace", msg.Keyspace,
				"batch", msg.Type.String(),
			},
		})
	case *message.Register:
		e.Audit.OnQuery(e.Context, e.sessionCtx, common.Query{
			Query: msg.String(),
		})
	case *message.Revise:
		// TODO(jakule): investigate this package. Looks like something only available in the enterprise edition.
		return trace.NotImplemented("revise package is not supported")
	default:
		return trace.BadParameter("received a message with unexpected type %T", body.Body.Message)
	}

	return nil
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
			// TODO(jakule): If protocol v5 then the message needs to be wrapped in a segment.
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
