package api

import (
	"crypto/tls"
	"io/ioutil"
	"os"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"golang.org/x/net/http2"
)

func LoadTLSConfig(certPath, keyPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tlsConfig tls.Config
	tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	tlsConfig.NextProtos = []string{http2.NextProtoTLS}
	utils.SetupTLSConfig(&tlsConfig, nil)
	tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	return &tlsConfig, nil
}

// InitSelfSignedHTTPSCert generates and self-signs a TLS key+cert pair for https connection
// to the proxy server.
func InitSelfSignedHTTPSCert(certPath, keyPath string) (err error) {
	// return the existing pair if they have already been generated:
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return trace.Wrap(err, "unrecognized error reading certs")
	}

	creds, err := utils.GenerateSelfSignedCert([]string{"localhost"})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := ioutil.WriteFile(keyPath, creds.PrivateKey, 0600); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	if err := ioutil.WriteFile(certPath, creds.Cert, 0600); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	return nil
}
