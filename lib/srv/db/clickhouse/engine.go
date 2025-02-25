/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
		e.Log.ErrorContext(e.Context, "Unsupported protocol", "protocol", protocol, "error", err)
	}
}

func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     sessionCtx.Database,
		DatabaseUser: sessionCtx.DatabaseUser,
		DatabaseName: sessionCtx.DatabaseName,
	})
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
