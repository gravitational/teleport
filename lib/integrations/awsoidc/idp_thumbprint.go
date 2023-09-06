/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsoidc

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib"
)

// ThumbprintIdP returns the thumbprint as required by AWS when adding an OIDC Identity Provider.
// This is documented here:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html
// Returns the thumbprint of the top intermediate CA that signed the TLS cert used to serve HTTPS requests.
// In case of a self signed certificate, then it returns the thumbprint of the TLS cert itself.
func ThumbprintIdP(ctx context.Context, publicAddress string) (string, error) {
	issuer, err := IssuerFromPublicAddress(publicAddress)
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
	return fmt.Sprintf("%x", thumbprint), nil
}
