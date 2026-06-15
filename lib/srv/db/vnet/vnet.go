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

// Package vnet contains VNet-related helpers shared between the auth server
// and the database agent.
package vnet

import (
	"crypto/sha256"
	"encoding/base32"
)

// base32hex is the "Extended Hex Alphabet" defined in RFC 4648 but with
// lowercase letters and no padding.
var base32hex = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

// DNSName returns a DNS-safe, deterministic name derived from the database name.
// used by VNet for database FQDN resolution.
func DNSName(dbName string) string {
	sum := sha256.Sum256([]byte(dbName))
	return base32hex.EncodeToString(sum[:8])
}
