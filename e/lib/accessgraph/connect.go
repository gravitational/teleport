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

// ClientCredentials holds TLS client credentials for connecting to the access graph service.
type ClientCredentials struct {
	// Cert is the PEM-encoded TLS certificate used to authenticate to TAG
	CertPEM []byte
	// Key is the PEM-encoded private key for Cert
	KeyPEM []byte
}

// NewAccessGraphClient returns a new access graph service client.
func NewAccessGraphClient(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	credsOpt, err := grpcCredentials(config, creds)
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

// NewAccessGraphClientWithCert returns a new access graph service client.
func NewAccessGraphClientWithCert(ctx context.Context, config ServiceClientConfig, creds tls.Certificate, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opt, err := grpcCredentialsWithCert(config, creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := dial(ctx, config.Addr, append(opts, opt)...)
	return conn, trace.Wrap(err)
}

// grpcCredentials returns a grpc.DialOption configured with TLS credentials.
func grpcCredentials(config ServiceClientConfig, creds ClientCredentials) (grpc.DialOption, error) {
	cert, err := tls.X509KeyPair(
		creds.CertPEM,
		creds.KeyPEM,
	)
	if err != nil {
		return nil, trace.Wrap(err, "cannot parse keypair")
	}
	return grpcCredentialsWithCert(config, cert)
}

func grpcCredentialsWithCert(config ServiceClientConfig, cert tls.Certificate) (grpc.DialOption, error) {
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
		Certificates: []tls.Certificate{
			cert,
		},
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: config.Insecure,
		RootCAs:            pool,
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}
