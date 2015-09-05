package srv

import (
	"crypto/tls"
	"net/http"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
)

func ListenAndServeTLS(address string, handler http.Handler,
	certFile, keyFile string) error {

	tlsConfig, err := CreateTLSConfiguration(certFile, keyFile)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}

	return http.Serve(listener, handler)
}

func CreateTLSConfiguration(certFile, keyFile string) (*tls.Config, error) {
	config := &tls.Config{}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
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
	config.ClientSessionCache = tls.NewLRUClientSessionCache(DefaultLRUCapacity)

	return config, nil
}

const DefaultLRUCapacity = 1024
