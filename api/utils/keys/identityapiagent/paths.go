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

package identityapiagent

import (
	"path/filepath"

	"github.com/gravitational/trace"
)

const (
	// SocketFileName is the unix socket used by the identity-api signer service.
	SocketFileName = ".identity-api.sock"
	// CertFileName is the pinned self-signed server certificate.
	CertFileName = ".identity-api-cert.pem"
)

// PathsFromIdentityFile returns the socket and certificate paths for the given
// Teleport identity file.
func PathsFromIdentityFile(identityPath string) (socketPath, certPath string, err error) {
	if identityPath == "" {
		return "", "", trace.BadParameter("identity path must be provided")
	}

	absIdentityPath, err := filepath.Abs(identityPath)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	dir := filepath.Dir(absIdentityPath)
	return filepath.Join(dir, SocketFileName), filepath.Join(dir, CertFileName), nil
}
