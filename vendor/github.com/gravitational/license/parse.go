/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package license

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/license/constants"

	"github.com/gravitational/trace"
)

// ParseLicensePEM parses license PEM, parses payload on demand
func ParseLicensePEM(pem []byte) (*License, error) {
	certPEM, keyPEM, err := SplitPEM([]byte(pem))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certificateBytes, _, err := parseCertificatePEM(string(pem))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certificate, err := x509.ParseCertificate(certificateBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rawPayload, err := getRawPayloadFromX509(certificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	license := License{
		Cert:       certificate,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		RawPayload: rawPayload,
	}

	return &license, nil

}

// ParseX509 parses the license from the provided x509 certificate
func ParseX509(cert *x509.Certificate) (*License, error) {
	rawPayload, err := getRawPayloadFromX509(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	license := License{
		Cert:       cert,
		RawPayload: rawPayload,
	}

	return &license, nil
}

// MakeTLSCert takes the provided license and makes a TLS certificate
// which is the format used by Go servers
func MakeTLSCert(license License) (*tls.Certificate, error) {
	tlsCert, err := tls.X509KeyPair(license.CertPEM, license.KeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tlsCert, nil
}

// MakeTLSConfig builds a client TLS config from the supplied license
func MakeTLSConfig(license License) (*tls.Config, error) {
	tlsCert, err := MakeTLSCert(license)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{*tlsCert},
	}, nil
}

// getRawPayloadFromX509 returns the payload in the extension of the
// provided x509 certificate
func getRawPayloadFromX509(cert *x509.Certificate) ([]byte, error) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(constants.LicenseASN1ExtensionID) {
			return ext.Value, nil
		}
	}
	return nil, trace.NotFound(
		"certificate does not contain extension with license payload")
}

// parseCertificatePEM parses the concatenated certificate/private key in PEM format
// and returns certificate and private key in decoded DER ASN.1 structure
func parseCertificatePEM(certPEM string) ([]byte, []byte, error) {
	var certificateBytes, privateBytes []byte
	block, rest := pem.Decode([]byte(certPEM))
	for block != nil {
		switch block.Type {
		case constants.CertificatePEMBlock:
			certificateBytes = block.Bytes
		case constants.RSAPrivateKeyPEMBlock:
			privateBytes = block.Bytes
		}
		// parse the next block
		block, rest = pem.Decode(rest)
	}
	if len(certificateBytes) == 0 || len(privateBytes) == 0 {
		return nil, nil, trace.BadParameter("could not parse the license")
	}
	return certificateBytes, privateBytes, nil
}

// SplitPEM splits the provided PEM data that contains concatenated cert and key
// (in any order) into cert PEM and key PEM respectively. Returns an error if
// any of them is missing
func SplitPEM(pemData []byte) (certPEM []byte, keyPEM []byte, err error) {
	block, rest := pem.Decode(pemData)
	for block != nil {
		switch block.Type {
		case constants.CertificatePEMBlock:
			certPEM = pem.EncodeToMemory(block)
		case constants.RSAPrivateKeyPEMBlock:
			keyPEM = pem.EncodeToMemory(block)
		}
		block, rest = pem.Decode(rest)
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, nil, trace.BadParameter("cert or key PEM data is missing")
	}
	return certPEM, keyPEM, nil
}
