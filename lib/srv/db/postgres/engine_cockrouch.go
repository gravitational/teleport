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

package postgres

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

func init() {
	common.RegisterEngine(newCockroachEngine, defaults.ProtocolCockroachDB)
}

func newCockroachEngine(ec common.EngineConfig) common.Engine {
	return &cockroachEngine{
		EngineConfig: ec,
	}
}

type cockroachEngine struct {
	EngineConfig common.EngineConfig
	engine       common.Engine
}

func (e *cockroachEngine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	conn, grpcConn, err := isGRPCConnection(clientConn)
	if err != nil {
		return trace.Wrap(err)
	}

	if grpcConn {
		e.engine = &grpcEngine{
			EngineConfig: e.EngineConfig,
		}
		return e.engine.InitializeConnection(conn, sessionCtx)
	}

	e.engine = &pgWireEngine{
		EngineConfig: e.EngineConfig,
	}
	return trace.Wrap(e.engine.InitializeConnection(conn, sessionCtx))
}

func (e *cockroachEngine) SendError(err error) {
	e.engine.SendError(err)
}

func (e *cockroachEngine) HandleConnection(ctx context.Context, session *common.Session) error {
	return trace.Wrap(e.engine.HandleConnection(ctx, session))
}

type grpcEngine struct {
	EngineConfig common.EngineConfig
	clientConn   net.Conn
}

func (e *grpcEngine) InitializeConnection(clientConn net.Conn, _ *common.Session) error {
	e.clientConn = clientConn
	return nil
}

func (e *grpcEngine) HandleConnection(ctx context.Context, session *common.Session) error {
	err := checkAccess(ctx, e.EngineConfig, session)
	if err != nil {
		return trace.Wrap(err)
	}

	config, err := getConnectConfig(ctx, e.EngineConfig, session)
	if err != nil {
		return trace.Wrap(err)
	}
	e.EngineConfig.Audit.OnSessionStart(ctx, session, nil)
	defer e.EngineConfig.Audit.OnSessionEnd(ctx, session)

	serverConn, err := tls.Dial("tcp", session.Database.GetURI(), config.TLSConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(smallinsky) In order to inspect cockroachDB gRPC audit events implement transparent gRPC forwarding proxy.
	if err := utils.ProxyConn(ctx, e.clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (e *grpcEngine) SendError(_ error) {
}
