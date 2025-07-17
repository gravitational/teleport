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

package relayapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"slices"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	relayv1alpha "github.com/gravitational/teleport/api/gen/proto/go/teleport/relay/v1alpha"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/tlsca"
)

type DiscoverParams struct {
	GetCertificate func() (*tls.Certificate, error)
	GetPool        func() (*x509.CertPool, error)
	Ciphersuites   []uint16

	Target string
}

// Discover returns configuration and connectivity information from a Relay and
// its group, given an API endpoint and some cluster authentication data. As the
// discover API is intended to only be used sporadically and with what should be
// a fresh server behind a load balancer every time, this function establishes a
// brand new connection and disposes of it before returning.
func Discover(ctx context.Context, params DiscoverParams) (*relayv1alpha.DiscoverResponse, error) {
	if params.GetCertificate == nil {
		return nil, trace.BadParameter("missing GetCertificate")
	}
	cert, err := params.GetCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if params.GetPool == nil {
		return nil, trace.BadParameter("missing GetPool")
	}
	pool, err := params.GetPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return cert, nil
		},
		RootCAs: pool,

		VerifyConnection: func(cs tls.ConnectionState) error {
			id, err := tlsca.FromSubject(cs.PeerCertificates[0].Subject, cs.PeerCertificates[0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}

			if !slices.Contains(id.Groups, string(types.RoleRelay)) &&
				!slices.Contains(id.SystemRoles, string(types.RoleRelay)) {
				return trace.BadParameter("dialed server is not a relay (roles %+q, system roles %+q)", id.Groups, id.SystemRoles)
			}

			return nil
		},

		NextProtos: []string{"h2"},
		ServerName: "",

		CipherSuites: params.Ciphersuites,
		MinVersion:   tls.VersionTLS12,
	}

	cc, err := grpc.NewClient(params.Target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(
			metadata.UnaryClientInterceptor,
			interceptors.GRPCClientUnaryErrorInterceptor,
		),
		grpc.WithChainStreamInterceptor(
			metadata.StreamClientInterceptor,
			interceptors.GRPCClientStreamErrorInterceptor,
		),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer cc.Close()

	clt := relayv1alpha.NewDiscoveryServiceClient(cc)
	return clt.Discover(ctx, &relayv1alpha.DiscoverRequest{})
}
