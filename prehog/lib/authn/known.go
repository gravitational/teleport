package authn

import (
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/asn1"

	"github.com/google/uuid"
)

//go:embed known.pem
var knownPEM []byte

var KnownLicenseCAs = func() *x509.CertPool {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(knownPEM) {
		panic("invalid license CAs")
	}
	return pool
}()

var NoClientCAs = func() *x509.CertPool {
	pool := x509.NewCertPool()
	// this should be a felony
	s, err := asn1.Marshal((pkix.Name{SerialNumber: uuid.NewString()}).ToRDNSequence())
	if err != nil {
		panic(err)
	}
	pool.AddCert(&x509.Certificate{RawSubject: s})
	return pool
}()
