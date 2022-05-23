package client

import (
	"crypto/tls"
	"crypto/x509"
)

// NewClientTLSConfig: generate TLS config for client side
// if insecureSkipVerify is set to true, serverName will not be validated
func NewClientTLSConfig(caPem, certPem, keyPem []byte, insecureSkipVerify bool, serverName string) *tls.Config {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPem) {
		panic("failed to add ca PEM")
	}

	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		panic(err)
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		InsecureSkipVerify: insecureSkipVerify,
		ServerName:         serverName,
	}
	return config
}
