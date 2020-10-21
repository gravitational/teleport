package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"os"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// connectClient establishes a gRPC client connected to an auth server.
func connectClient() (*auth.Client, error) {
	tlsConfig, err := LoadTLSConfig("certs/api-admin.crt", "certs/api-admin.key", "certs/api-admin.cas")
	if err != nil {
		log.Fatalf("Failed to setup TLS config: %v", err)
	}

	// replace 127.0.0.1:3025 (default) with your auth server address
	authServerAddr := utils.MustParseAddrList("127.0.0.1:3025")
	clientConfig := auth.ClientConfig{Addrs: authServerAddr, TLS: tlsConfig}

	return auth.NewTLSClient(clientConfig)
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
		return nil, trace.Wrap(err)
	}
	caCerts, err := ioutil.ReadAll(caFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCerts); !ok {
		return nil, trace.BadParameter("invalid CA cert PEM")
	}
	return pool, nil
}
