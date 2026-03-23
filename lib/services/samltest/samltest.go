/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
package samltest

import (
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

// CreateTestEntityDescriptor returns a XML entity descriptor containing signing certificates
// with expiries from the provided ttls.
func CreateTestEntityDescriptor(t *testing.T, ttls []time.Duration) string {
	t.Helper()

	var certs []string
	for _, ttl := range ttls {
		_, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{}, nil, ttl)
		require.NoError(t, err)

		block, _ := pem.Decode(certPEM)
		certs = append(certs, fmt.Sprintf(
			`<md:KeyDescriptor use="signing"><ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor>`,
			base64.StdEncoding.EncodeToString(block.Bytes),
		))
	}

	return fmt.Sprintf(
		`<?xml version="1.0"?><md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test"><md:IDPSSODescriptor>%s</md:IDPSSODescriptor></md:EntityDescriptor>`,
		strings.Join(certs, ""),
	)
}
