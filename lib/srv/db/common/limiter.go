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
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// NoopLimiter implements Limiter interface without applying any limit.
type NoopLimiter struct{}

func (NoopLimiter) RegisterClientIP(conn net.Conn) (func(), string, error) {
	clientIP, err := utils.ClientIPFromConn(conn)
	return func() {}, clientIP, trace.Wrap(err)
}

func (NoopLimiter) RegisterIdentity(context.Context, *ProxyContext) (func(), error) {
	return func() {}, nil
}
