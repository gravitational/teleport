// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package relaytunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/lib/tlsca"
)

type ServerConfig struct {
	GetCertificate func(ctx context.Context) (*tls.Certificate, error)
	GetPool        func(ctx context.Context) (*x509.CertPool, error)
	Ciphersuites   []uint16
}

func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.GetCertificate == nil {
		return nil, trace.BadParameter("missing GetCertificate")
	}
	if cfg.GetPool == nil {
		return nil, trace.BadParameter("missing GetPool")
	}
	return &Server{
		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,
	}, nil
}

type Server struct {
	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16
}

func (s *Server) GRPCServerCredentials() credentials.TransportCredentials {
	return &grpcServerCredentials{
		tunnelServer: s,

		getCertificate: s.getCertificate,
		getPool:        s.getPool,
		ciphersuites:   s.ciphersuites,
	}
}

type grpcServerCredentials struct {
	tunnelServer *Server

	getCertificate func(ctx context.Context) (*tls.Certificate, error)
	getPool        func(ctx context.Context) (*x509.CertPool, error)
	ciphersuites   []uint16
}

// ClientHandshake implements [credentials.TransportCredentials].
func (*grpcServerCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	_ = rawConn.Close()
	return nil, nil, trace.NotImplemented("these transport credentials can only be used as a server")
}

// OverrideServerName implements implements [credentials.TransportCredentials].
func (*grpcServerCredentials) OverrideServerName(string) error {
	return nil
}

// Clone implements implements [credentials.TransportCredentials].
func (s *grpcServerCredentials) Clone() credentials.TransportCredentials {
	// s is immutable so there's no need to copy anything
	return s
}

// Info implements implements [credentials.TransportCredentials].
func (s *grpcServerCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
	}
}

// ServerHandshake implements implements [credentials.TransportCredentials].
func (s *grpcServerCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cert, err := s.getCertificate(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}
	pool, err := s.getPool(ctx)
	if err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	var clientID *tlsca.Identity
	tlsConfig := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				// client tried to connect with no ALPN (or with http/1.1 in its
				// protocol list because of an undocumented behavior of the
				// crypto/tls server handshake)
				return trace.NotImplemented("missing ALPN in TLS ClientHello")
			}
			if len(cs.VerifiedChains) < 1 {
				return trace.AccessDenied("missing or invalid client certificate")
			}

			if cs.NegotiatedProtocol == "h2" {
				return nil
			}

			id, err := tlsca.FromSubject(cs.VerifiedChains[0][0].Subject, cs.VerifiedChains[0][0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}
			clientID = id

			return nil
		},
		NextProtos: []string{yamuxTunnelALPN, "h2"},

		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,

		InsecureSkipVerify: false,

		MinVersion:             tls.VersionTLS12,
		CipherSuites:           s.ciphersuites,
		SessionTicketsDisabled: true,
	}

	tlsConn := tls.Server(rawConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		_ = tlsConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	cs := tlsConn.ConnectionState()
	if cs.NegotiatedProtocol == yamuxTunnelALPN {
		// TODO(espadolini): handle the actual tunnel connection, using clientID
		// for authz
		_ = tlsConn.Close()
		_ = clientID
		return nil, nil, credentials.ErrConnDispatched
	}
	tlsInfo := credentials.TLSInfo{
		State: cs,
		CommonAuthInfo: credentials.CommonAuthInfo{
			SecurityLevel: credentials.PrivacyAndIntegrity,
		},
	}

	return tlsConn, tlsInfo, nil
}
