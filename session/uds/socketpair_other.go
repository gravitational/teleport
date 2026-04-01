//go:build !unix

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

package uds

import (
	"errors"
	"net"

	"github.com/gravitational/trace"
)

var nonUnixErr = errors.New("socket pair not available on non-unix platform")

// NewSocketpair creates a unix socket pair, returning the halves as
// [*net.UnixConn]s.
func NewSocketpair(t SocketType) (left, right *net.UnixConn, err error) {
	return nil, nil, trace.Wrap(nonUnixErr)
}
