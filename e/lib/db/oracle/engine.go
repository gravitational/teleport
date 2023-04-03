/*
Copyright 2023 Gravitational, Inc.

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

package oracle

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new Oracle engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements the Oracle database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the Oracle database instance.
//
// Implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// proxyConn is a client connection.
	conn               net.Conn
	clientConn         *protocol.Conn
	serverNameReceived bool
}

// InitializeConnection initializes the engine with client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, _ *common.Session) error {
	e.conn = clientConn
	return nil
}

// SendError sends an error to connected client in the Oracle understandable format.
func (e *Engine) SendError(err error) {
	// TODO: Investigate way to propagate Oracle error message.
	if err != nil && !utils.IsOKNetworkError(err) {
		e.Log.WithError(err).Error("Oracle connection error")
	}
}

// HandleConnection processes the connection from Oracle proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	defer e.conn.Close()

	clientConn := protocol.NewClientConn(e.conn)
	e.clientConn = clientConn

	err := e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	serverConn, err := e.connectToOracleDB(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	if err := e.handleClientServerConn(ctx, sessionCtx, clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *Engine) connectToOracleDB(ctx context.Context, sessionCtx *common.Session) (*protocol.Conn, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverConn, err := protocol.NewServerConn(sessionCtx.Database.GetURI(), tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return serverConn, nil
}

func (e *Engine) handleClientConn(sessCtx *common.Session, clientConn, serverConn *protocol.Conn) error {
	defer clientConn.Close()
	defer serverConn.Close()
	for {
		p, err := clientConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}
		switch t := p.(type) {
		case *protocol.ConnectPacket:
			if t.ServerName != "" {
				e.serverNameReceived = true
			}
			if sessCtx.Identity.RouteToDatabase.Database != t.ServerName {
				return trace.BadParameter("mismatch between TLS identity database name and Oracle Connect Packet ServerName")
			}
		}
		if err := serverConn.WritePacket(p); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) handleServerConn(ctx *common.Session, clientConn, serverConn *protocol.Conn) error {
	defer serverConn.Close()
	defer clientConn.Close()
	for {
		packet, err := serverConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}
		if !serverConn.ConnAccepted() {
			switch t := packet.(type) {
			case *protocol.RefusePacket:
				// Debug connection errors before accepted phase in order to troubleshoot misconfiguration issues.
				e.Log.WithField("message", t.Message).Warn("Received Refuse Packet from server.")
			case *protocol.AcceptPacket:
				if !e.serverNameReceived {
					return trace.BadParameter("server name package not received")
				}
			}
		}
		if err = clientConn.WritePacket(packet); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) handleClientServerConn(ctx context.Context, sessionCtx *common.Session, clientConn, serverConn *protocol.Conn) error {
	errC := make(chan error, 2)
	go func() {
		err := e.handleClientConn(sessionCtx, clientConn, serverConn)
		errC <- trace.Wrap(err, "client done")
	}()
	go func() {
		var err = e.handleServerConn(sessionCtx, clientConn, serverConn)
		errC <- trace.Wrap(err, "server done")
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case err := <-errC:
			if err != nil && !utils.IsOKNetworkError(errors.Unwrap(err)) && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)
}

func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.DatabaseRoleMatchers(
		sessionCtx.Database,
		sessionCtx.DatabaseUser,
		sessionCtx.DatabaseName,
	)
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}
