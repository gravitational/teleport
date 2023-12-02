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

package dbutils

import (
	"crypto/tls"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tlsca"
)

// IsDatabaseConnection inspects the TLS connection state and returns true
// if it's a database access connection as determined by the decoded
// identity from the client certificate.
func IsDatabaseConnection(state tls.ConnectionState) (bool, error) {
	// VerifiedChains must be populated after the handshake.
	if len(state.VerifiedChains) < 1 || len(state.VerifiedChains[0]) < 1 {
		return false, nil
	}
	identity, err := tlsca.FromSubject(state.VerifiedChains[0][0].Subject,
		state.VerifiedChains[0][0].NotAfter)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return identity.RouteToDatabase.ServiceName != "", nil
}
