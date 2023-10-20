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

package clickhouse

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{EngineConfig: ec}
}

type Engine struct {
	common.EngineConfig
	clientConn net.Conn
	sessionCtx *common.Session
}

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
	return nil
}

func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	if err := e.checkAccess(ctx, sessionCtx); err != nil {
		return trace.Wrap(err)
	}

	if sessionCtx.Identity.RouteToDatabase.Username == "" {
		return trace.BadParameter("database username required for ClickHouse")
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	switch protocol := sessionCtx.Database.GetProtocol(); protocol {
	case defaults.ProtocolClickHouseHTTP:
		return trace.Wrap(e.handleHTTPConnection(ctx, sessionCtx))
	case defaults.ProtocolClickHouse:
		return trace.Wrap(e.handleNativeConnection(ctx, sessionCtx))
	default:
		return trace.BadParameter("protocol %s is not supported", protocol)
	}
}

func (e *Engine) SendError(err error) {
	if err == nil || utils.IsOKNetworkError(err) {
		return
	}

	switch protocol := e.sessionCtx.Database.GetProtocol(); protocol {
	case defaults.ProtocolClickHouseHTTP:
		e.sendErrorHTTP(err)
	case defaults.ProtocolClickHouse:
		e.sendErrorNative(err)
	default:
		e.Log.Error("Unsupported protocol %q", protocol)
	}
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
	}
	return trace.Wrap(err)
}
