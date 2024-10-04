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

package peer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"

	"github.com/gravitational/teleport"
)

type QUICServerConfig struct {
	Log           *slog.Logger
	ClusterDialer ClusterDialer

	CipherSuites   []uint16
	GetCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	GetClientCAs   func(*tls.ClientHelloInfo) (*x509.CertPool, error)
}

func (c *QUICServerConfig) checkAndSetDefaults() error {
	if c.Log == nil {
		c.Log = slog.Default()
	}
	c.Log = c.Log.With(
		teleport.ComponentKey,
		teleport.Component(teleport.ComponentProxy, "qpeer"),
	)

	if c.ClusterDialer == nil {
		return trace.BadParameter("missing cluster dialer")
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

func NewQUICServer(cfg QUICServerConfig) (*QUICServer, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	panic("QUIC proxy peering is not implemented")
}

func (s *QUICServer) Serve(t *quic.Transport) error {
	panic("QUIC proxy peering is not implemented")
}

func (s *QUICServer) Close() error {
	panic("QUIC proxy peering is not implemented")
}

func (s *QUICServer) Shutdown(ctx context.Context) error {
	panic("QUIC proxy peering is not implemented")
}
