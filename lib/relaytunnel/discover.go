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

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
)

//nolint:unused // used by StaticDiscoverServiceServer
type unimplementedDiscoveryServiceServer = relaytunnelv1alpha.UnimplementedDiscoveryServiceServer

// StaticDiscoverServiceServer is a [relaytunnelv1alpha.DiscoveryServiceServer]
// implementation that responds with fixed data to the Discover rpc.
type StaticDiscoverServiceServer struct {
	_ struct{} // prevent unkeyed literals

	unimplementedDiscoveryServiceServer //nolint:unused // required and used by grpc-go

	RelayGroup            string
	TargetConnectionCount int32
}

var _ relaytunnelv1alpha.DiscoveryServiceServer = (*StaticDiscoverServiceServer)(nil)

// Discover implements [relaytunnelv1alpha.DiscoveryServiceServer].
func (d *StaticDiscoverServiceServer) Discover(ctx context.Context, req *relaytunnelv1alpha.DiscoverRequest) (*relaytunnelv1alpha.DiscoverResponse, error) {
	return &relaytunnelv1alpha.DiscoverResponse{
		RelayGroup:            d.RelayGroup,
		TargetConnectionCount: d.TargetConnectionCount,
	}, nil
}

type DiscoverParams struct {
	GetCertificate func() (*tls.Certificate, error)
	GetPool        func() (*x509.CertPool, error)
	Ciphersuites   []uint16

	Target string
}

// discover returns configuration and connectivity information from a Relay and
// its group, given an API endpoint and some cluster authentication data. As the
// discover API is intended to only be used sporadically and with what should be
// a fresh server behind a load balancer every time, this function establishes a
// brand new connection and disposes of it before returning.
func discover(ctx context.Context, params DiscoverParams) (*relaytunnelv1alpha.DiscoverResponse, error) {
	if params.GetCertificate == nil {
		return nil, trace.BadParameter("missing GetCertificate")
	}
	if params.GetPool == nil {
		return nil, trace.BadParameter("missing GetPool")
	}

	cert, err := params.GetCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
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

		// the [credentials.NewTLS] transport credentials will take care of SNI
		// and ALPN
		NextProtos: nil,
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

	clt := relaytunnelv1alpha.NewDiscoveryServiceClient(cc)

	resp, err := clt.Discover(ctx, &relaytunnelv1alpha.DiscoverRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// TODO(espadolini): remove once the function is actually used
var _ = discover
