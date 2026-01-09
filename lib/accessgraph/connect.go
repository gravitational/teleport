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
	"crypto/tls"
	"crypto/x509"
	"strings"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/stats"

	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessGraphClientConfig is the configuration for the access graph service client.
type AccessGraphClientConfig struct {
	// Addr is the address of the access graph service.
	Addr string
	// CA is the PEM-encoded CA certificate used to verify the access graph GRPC connection.
	CA []byte
	// Insecure is true if the access graph GRPC connection should be insecure.
	// Do not use in production.
	Insecure bool
	// CipherSuites is the list of TLS cipher suites to use for the connection.
	CipherSuites []uint16
	// ClientCredentials is a function that returns the client TLS certificate.
	ClientCredentials ClientCredentialsGetter
}

// GRPCClientConnInterface is an interface that extends grpc.ClientConnInterface
// with the ability to wait for state changes.
type GRPCClientConnInterface interface {
	grpc.ClientConnInterface
	WaitForStateChange(ctx context.Context, sourceState connectivity.State) bool
}

type Connector interface {
	Role() types.SystemRole
	ClientGetCertificate() (*tls.Certificate, error)
}

// AccessGraphClientGetterForConnector is a function that returns an AccessGraphClientGetter for a given connector.
// This allows different parts of the system to get access graph clients with different certificates based on the connector.
type AccessGraphClientGetterForConnector = func(connector Connector) AccessGraphClientGetter

// AccessGraphClientGetter is a function that returns a new access graph service client connection.
type AccessGraphClientGetter = func(ctx context.Context) (GRPCClientConnInterface, error)

// ClientCredentialsGetter is a function that returns the client TLS certificate.
type ClientCredentialsGetter = func() (*tls.Certificate, error)

// NewAccessGraphClient returns a new access graph service client.
func NewAccessGraphClient(ctx context.Context, config AccessGraphClientConfig, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	credsOpt, err := grpcCredentials(config, config.ClientCredentials)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otelOpt := grpc.WithStatsHandler(otelgrpc.NewClientHandler(
		otelgrpc.WithFilter(filters.All(
			filters.Not(filters.HealthCheck()),
			func(i *stats.RPCTagInfo) bool {
				return !strings.Contains(i.FullMethodName, "Stream")
			},
		)),
	))

	opts = append([]grpc.DialOption{credsOpt, otelOpt}, opts...)
	conn, err := dial(ctx, config.Addr, opts...)
	return conn, trace.Wrap(err)
}

func dial(ctx context.Context, addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	const maxMessageSize = 50 * 1024 * 1024 // 50MB
	opts = append(opts,
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMessageSize),
			grpc.MaxCallSendMsgSize(maxMessageSize),
		),
	)

	conn, err := grpc.DialContext(ctx, addr, opts...)
	return conn, trace.Wrap(err)
}

// grpcCredentials returns a grpc.DialOption configured with TLS credentials.
func grpcCredentials(config AccessGraphClientConfig, getCreds ClientCredentialsGetter) (grpc.DialOption, error) {
	if getCreds == nil {
		return nil, trace.BadParameter("missing credential getter")
	}

	var pool *x509.CertPool
	if len(config.CA) > 0 {
		pool = x509.NewCertPool()
		if !pool.AppendCertsFromPEM(config.CA) {
			return nil, trace.BadParameter("failed to append CA certificate to pool")
		}
	}

	tlsConfig := utils.TLSConfig(config.CipherSuites)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return getCreds()
	}
	tlsConfig.InsecureSkipVerify = config.Insecure
	tlsConfig.RootCAs = pool

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}
