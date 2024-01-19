package accessgraph

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	opt, err := grpcCredentials(config, creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := grpc.DialContext(ctx, config.Addr, append(opts, opt)...)
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
