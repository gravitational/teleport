package srv

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

func ListenAndServeTLS(address string, handler http.Handler,
	cert, key string) error {

	tlsConfig, err := CreateTLSConfiguration(cert, key)
	if err != nil {
		return trace.Wrap(err)
	}

	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	return http.Serve(listener, handler)
}

func CreateTLSConfiguration(certString, keyString string) (*tls.Config, error) {
	config := &tls.Config{}

	certPEM, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		return nil, trace.Wrap(err, "expected base64 encoded cert")
	}

	keyPEM, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, trace.Wrap(err, "expected base64 encoded key")
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config.Certificates = []tls.Certificate{cert}

	config.CipherSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,

		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,

		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	}

	config.MinVersion = tls.VersionTLS12
	config.SessionTicketsDisabled = false
	config.ClientSessionCache = tls.NewLRUClientSessionCache(
		DefaultLRUCapacity)

	return config, nil
}

const DefaultLRUCapacity = 1024
