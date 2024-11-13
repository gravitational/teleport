// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package internal

import (
	"context"
	"crypto/x509"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

// VerifyPeerCertificateIsProxy is a function usable as a
// [tls.Config.VerifyPeerCertificate] callback to enforce that the connected TLS
// client is using Proxy credentials.
func VerifyPeerCertificateIsProxy(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(verifiedChains) < 1 {
		return trace.AccessDenied("missing client certificate (this is a bug)")
	}

	clientCert := verifiedChains[0][0]
	clientIdentity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	if !slices.Contains(clientIdentity.Groups, string(types.RoleProxy)) {
		return trace.AccessDenied("expected Proxy client credentials")
	}
	return nil
}

// VerifyPeerCertificateIsSpecificProxy returns a function usable as a
// [tls.Config.VerifyPeerCertificate] callback to enforce that the connected TLS
// server is using Proxy credentials and has the expected host ID.
func VerifyPeerCertificateIsSpecificProxy(peerID string) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) < 1 {
			return trace.AccessDenied("missing server certificate (this is a bug)")
		}

		clientCert := verifiedChains[0][0]
		clientIdentity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
		if err != nil {
			return trace.Wrap(err)
		}

		if !slices.Contains(clientIdentity.Groups, string(types.RoleProxy)) {
			return trace.AccessDenied("expected Proxy server credentials")
		}

		if clientIdentity.Username != peerID {
			return trace.Wrap(WrongProxyError{})
		}
		return nil
	}
}

// LogDuplicatePeer should be used to log a message if a proxy peering client
// connects to a Proxy that did not have the expected host ID.
func LogDuplicatePeer(ctx context.Context, log *slog.Logger, level slog.Level, args ...any) {
	const duplicatePeerMsg = "" +
		"Detected multiple Proxy Peers with the same public address when connecting to a Proxy which can lead to inconsistent state and problems establishing sessions. " +
		"For best results ensure that `peer_public_addr` is unique per proxy and not a load balancer."
	log.Log(ctx, level, duplicatePeerMsg, args...)
}

// WrongProxyError signals that a proxy peering client has connected to a Proxy
// that did not have the expected host ID.
type WrongProxyError struct{}

func (WrongProxyError) Error() string {
	return "connected to unexpected proxy"
}

func (e WrongProxyError) Unwrap() error {
	return &trace.AccessDeniedError{
		Message: e.Error(),
	}
}
