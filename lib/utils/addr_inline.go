// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package utils

import (
	"net"

	"github.com/gravitational/teleport/session/common/netutils"
)

//go:fix inline
type NetAddr = netutils.Addr

//go:fix inline
func ParseAddr(a string) (*netutils.Addr, error) { return netutils.ParseAddr(a) }

//go:fix inline
func MustParseAddr(a string) *netutils.Addr { return netutils.MustParseAddr(a) }

//go:fix inline
func FromAddr(a net.Addr) netutils.Addr { return netutils.FromAddr(a) }

//go:fix inline
func ParseHostPortAddr(hostport string, defaultPort int) (*netutils.Addr, error) {
	return netutils.ParseHostPortAddr(hostport, defaultPort)
}
