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

package accessgraph

import (
	"context"
	"crypto"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/trace"
)

// generateAccessGraphUserCerts re-signs the existing TLS key material with an
// Access Graph-specific usage so the request can be authenticated by the proxy.
func generateAccessGraphUserCerts(ctx context.Context, client *authclient.Client, signer crypto.Signer, username string) ([]byte, *proto.Certs, error) {
	tlsPublicKey, err := keys.MarshalPublicKey(signer.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(signer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		TLSPublicKey: tlsPublicKey,
		Username:     username,
		Expires:      time.Now().Add(time.Hour),
		Usage:        proto.UserCertsRequest_AccessGraphAPI,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return privateKeyPEM, certs, nil
}

// newAccessGraphTLSConfig builds a web-proxy TLS config and layers the
// Access Graph client certificate on top of it.
func newAccessGraphTLSConfig(serverName string, clientCert tls.Certificate) (*tls.Config, error) {
	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// existingTLSSigner extracts the signer backing the standard tctl TLS identity
// so Access Graph can reuse the same key material with a different cert usage.
func existingTLSSigner(tlsConfig *tls.Config) (crypto.Signer, error) {
	if len(tlsConfig.Certificates) > 0 {
		return signerFromTLSCertificate(&tlsConfig.Certificates[0])
	}
	if tlsConfig.GetClientCertificate == nil {
		return nil, trace.BadParameter("missing TLS client certificate")
	}
	fmt.Println("We made it here, since we do not have a certificate in the TLS config, but we do have a GetClientCertificate callback. Calling it to get the cert.")
	cert, err := tlsConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return signerFromTLSCertificate(cert)
}

// signerFromTLSCertificate normalizes the TLS certificate's private key into a
// crypto.Signer so it can be re-signed by GenerateUserCerts.
func signerFromTLSCertificate(cert *tls.Certificate) (crypto.Signer, error) {
	if cert == nil {
		return nil, trace.BadParameter("missing TLS client certificate")
	}
	signer, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("unsupported TLS private key type %T", cert.PrivateKey)
	}
	return signer, nil
}
