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

package subca

import (
	"crypto/x509"

	"github.com/gravitational/teleport/api/utils/tlsutils"
)

// ParseCertificateOverrideCertificate parses a PEM certificate defined within a
// CertAuthorityOverride spec.
//
// Exposed for uniformity/consistency with other layers.
func ParseCertificateOverrideCertificate(certPEM string) (*x509.Certificate, error) {
	return tlsutils.ParseCertificatePEMStrict([]byte(certPEM))
}
