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
	"crypto/tls"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

func (e *Engine) handleNativeConnection(ctx context.Context, sessionCtx *common.Session) error {
	u, err := url.Parse(sessionCtx.Database.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return trace.Wrap(err)
	}

	serverConn, err := tls.Dial("tcp", u.Host, tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	if err := utils.ProxyConn(ctx, e.clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) sendErrorNative(err error) {
	// TODO: Support clickhouse native wire protocol error messages.
	e.Log.DebugContext(e.Context, "Clickhouse client connection error.", "error", err)
}
