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

package internal

const (
	// HostCAPath is the default filename for the host CA certificate
	HostCAPath = "teleport-host-ca.crt"

	// UserCAPath is the default filename for the user CA certificate
	UserCAPath = "teleport-user-ca.crt"

	// DatabaseCAPath is the default filename for the database CA
	// certificate
	DatabaseCAPath = "teleport-database-ca.crt"

	// JWTSVIDPath is the name of the artifact that a JWT SVID will be written to.
	JWTSVIDPath = "jwt_svid"

	// IdentityFilePath is the name of the artifact that the identity will be written to.
	IdentityFilePath = "identity"

	// DefaultTLSPrefix is the default prefix in generated TLS certs.
	DefaultTLSPrefix = "tls"

	// RenewalRetryLimit is the number of permissible consecutive
	// failures in renewing credentials before the loop exits fatally.
	RenewalRetryLimit = 5
)

// Based on the default paths listed in
// https://github.com/spiffe/spiffe-helper/blob/main/README.md
const (
	SVIDPEMPath            = "svid.pem"
	SVIDKeyPEMPath         = "svid_key.pem"
	SVIDTrustBundlePEMPath = "svid_bundle.pem"
	SVIDCRLPemPath         = "svid_crl.pem"
)

const (
	// PEMBlockTypePrivateKey is the PEM block type for a PKCS 8 encoded private key.
	PEMBlockTypePrivateKey = "PRIVATE KEY"
	// PEMBlockTypeCertificate is the PEM block type for a DER encoded certificate.
	PEMBlockTypeCertificate = "CERTIFICATE"
)
