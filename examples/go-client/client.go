package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gravitational/teleport/api/client"
	auth "github.com/gravitational/teleport/lib/auth/client"
)

// connectClient establishes a gRPC connection to an auth server.
func connectClient() (*auth.Client, error) {
	tlsConfig, err := LoadTLSConfig("certs/api-admin.crt", "certs/api-admin.key", "certs/api-admin.cas")
	if err != nil {
		return nil, fmt.Errorf("Failed to setup TLS config: %v", err)
	}

	config := client.Config{
		// replace 127.0.0.1:3025 (default) with your auth server address
		Addrs:       []string{"127.0.0.1:3025"},
		Credentials: []client.Credentials{client.LoadTLS(tlsConfig)},
	}
	return auth.New(config)
}

// LoadTLSConfig loads and sets up client TLS config for authentication
func LoadTLSConfig(certPath, keyPath, rootCAsPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	caPool, err := LoadTLSCertPool(rootCAsPath)
	if err != nil {
		return nil, err
	}
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}
	return conf, nil
}

// LoadTLSCertPool is used to load root CA certs from file path.
func LoadTLSCertPool(path string) (*x509.CertPool, error) {
	caFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	caCerts, err := ioutil.ReadAll(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCerts); !ok {
		return nil, fmt.Errorf("invalid CA cert PEM")
	}
	return pool, nil
}
