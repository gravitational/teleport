/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"context"
)

const (
	// ConnHandlerSourceUnspecified indicates no source has been specified.
	ConnHandlerSourceUnspecified = "unspecified"
	// ConnHandlerSourceListener indicates the connection is from the TLS
	// routing port listener.
	ConnHandlerSourceListener = "listener"
	// ConnHandlerSourceWebConnUpgrade indicates the connection is from ALPN
	// upgrade web api.
	ConnHandlerSourceWebConnUpgrade = "web_conn_upgrade"
	// ConnHandlerSourceWebDB indicates the connection is from database access
	// via Web UI.
	ConnHandlerSourceWebDB = "web_db"
)

// WithConnHandlerSource adds connection source to the context.
func WithConnHandlerSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, handlerSourceKey{}, source)
}

// GetConnHandlerSource retrieves connection source from the context.
func GetConnHandlerSource(ctx context.Context) string {
	value, ok := ctx.Value(handlerSourceKey{}).(string)
	if !ok {
		return ConnHandlerSourceUnspecified
	}
	return value
}

type handlerSourceKey struct{}
