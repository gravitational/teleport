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

package awsoidc

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// ThumbprintIdP returns the thumbprint as required by AWS when adding an OIDC Identity Provider.
// This is documented here:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html
// Returns the thumbprint of the top intermediate CA that signed the TLS cert used to serve HTTPS requests.
// In case of a self signed certificate, then it returns the thumbprint of the TLS cert itself.
func ThumbprintIdP(ctx context.Context, publicAddress string) (string, error) {
	issuer, err := oidc.IssuerFromPublicAddress(publicAddress, "")
	if err != nil {
		return "", trace.Wrap(err)
	}

	addrURL, err := url.Parse(issuer)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// The port needs to be explicitly set because it uses tls.Dialer which doesn't have a default port.
	if addrURL.Port() == "" {
		addrURL.Host = addrURL.Host + ":443"
	}

	d := tls.Dialer{
		Config: &tls.Config{
			InsecureSkipVerify: lib.IsInsecureDevMode(),
		},
	}
	conn, err := d.DialContext(ctx, "tcp", addrURL.Host)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return "", trace.Errorf("failed to create a tls connection")
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", trace.Errorf("no certificates were provided")
	}

	// Get the last certificate of the chain
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html
	// > If you see more than one certificate, find the last certificate displayed (at the end of the command output).
	// > This contains the certificate of the top intermediate CA in the certificate authority chain.
	//
	// The guide above uses openssl but the expected list of certificates and their order is the same.

	lastCertificateIdx := len(certs) - 1
	cert := certs[lastCertificateIdx]
	thumbprint := sha1.Sum(cert.Raw)

	// Convert the thumbprint ([]bytes) into their hex representation.
	return hex.EncodeToString(thumbprint[:]), nil
}
