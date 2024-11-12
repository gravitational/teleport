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

package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"

	"github.com/gravitational/teleport"
	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
)

// QUICServerConfig holds the parameters for [NewQUICServer].
type QUICServerConfig struct {
	Log *slog.Logger
	// Dialer is the dialer used to open connections to agents on behalf
	// of the peer proxies. Required.
	Dialer peerdial.Dialer

	// CipherSuites is the set of TLS ciphersuites to be used by the server.
	//
	// Note: it won't actually have an effect, since QUIC always uses (the DTLS
	// equivalent of) TLS 1.3, and TLS 1.3 ciphersuites can't be configured in
	// crypto/tls, but for consistency's sake this should be passed along from
	// the agent configuration.
	CipherSuites []uint16
	// GetCertificate should return the server certificate at time of use. It
	// should be a certificate with the Proxy host role. Required.
	GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// GetClientCAs should return the certificate pool that should be used to
	// validate the client certificates of peer proxies; i.e., a pool containing
	// the trusted signers for the certificate authority of the local cluster.
	// Required.
	GetClientCAs func(*tls.ClientHelloInfo) (*x509.CertPool, error)
}

func (c *QUICServerConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.Default()
	}
	c.Log = c.Log.With(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentProxy, "qpeer"),
	)

	if c.Dialer == nil {
		return trace.BadParameter("missing Dialer")
	}

	if c.GetCertificate == nil {
		return trace.BadParameter("missing GetCertificate")
	}
	if c.GetClientCAs == nil {
		return trace.BadParameter("missing GetClientCAs")
	}

	return nil
}

// QUICServer is a proxy peering server that uses the QUIC protocol.
type QUICServer struct{}

// NewQUICServer returns a [QUICServer] with the given config.
func NewQUICServer(cfg QUICServerConfig) (*QUICServer, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	panic("QUIC proxy peering is not implemented")
}

// Serve opens a listener and serves incoming connection. Returns after calling
// Close or Shutdown.
func (s *QUICServer) Serve(t *quic.Transport) error {
	panic("QUIC proxy peering is not implemented")
}

// Close stops listening for incoming connections and ungracefully terminates
// all the existing ones.
func (s *QUICServer) Close() error {
	panic("QUIC proxy peering is not implemented")
}

// Shutdown stops listening for incoming connections and waits until the
// existing ones are closed or until the context expires. If the context
// expires, running connections are ungracefully terminated.
func (s *QUICServer) Shutdown(ctx context.Context) error {
	panic("QUIC proxy peering is not implemented")
}
