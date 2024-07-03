package accessgraph

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"strings"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc/filters"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/stats"

	"github.com/gravitational/teleport/api/metadata"
)

// ServiceClientConfig is the configuration for the access graph service client.
type ServiceClientConfig struct {
	// Addr is the address of the access graph service.
	Addr string
	// CA is the path to the CA certificate used to verify the access graph GRPC connection.
	CA string
	// Insecure is true if the access graph GRPC connection should be insecure.
	// Do not use in production.
	Insecure bool
}

type ClientCredentialsGetter = func() (*tls.Certificate, error)

// NewAccessGraphClient returns a new access graph service client.
func NewAccessGraphClient(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	credsOpt, err := grpcCredentials(config, getCreds)
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
	opts = append(opts,
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
	)

	conn, err := grpc.DialContext(ctx, addr, opts...)
	return conn, trace.Wrap(err)
}

// grpcCredentials returns a grpc.DialOption configured with TLS credentials.
func grpcCredentials(config ServiceClientConfig, getCreds ClientCredentialsGetter) (grpc.DialOption, error) {
	var pool *x509.CertPool
	if config.CA != "" {
		pool = x509.NewCertPool()
		caBytes, err := os.ReadFile(config.CA)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, trace.BadParameter("failed to append CA certificate to pool")
		}
	}

	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return getCreds()
		},
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: config.Insecure,
		RootCAs:            pool,
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}
