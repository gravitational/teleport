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

// Package sshca specifies interfaces for SSH certificate authorities
package sshca

import (
	"github.com/gravitational/teleport/lib/services"
)

// Authority implements minimal key-management facility for generating OpenSSH
// compatible public/private key pairs and OpenSSH certificates
type Authority interface {
	// GenerateHostCert takes the private key of the CA, public key of the new host,
	// along with metadata (host ID, node name, cluster name, roles, and ttl) and generates
	// a host certificate.
	GenerateHostCert(certParams services.HostCertParams) ([]byte, error)

	// GenerateUserCert generates user ssh certificate, it takes pkey as a signing
	// private key (user certificate authority)
	GenerateUserCert(certParams services.UserCertParams) ([]byte, error)
}
