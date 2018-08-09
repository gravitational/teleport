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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"

	"github.com/gravitational/license/constants"

	"github.com/gravitational/trace"
)

// ParseString parses the license from the provided string,
// assumes default payload is encoded in the license and parses it
func ParseString(pem string) (*License, error) {
	return ParseLicensePEM([]byte(pem), ParseOptions{ParsePayload: true})
}

// ParseOptions specifies parsing and validation options
type ParseOptions struct {
	// ParsePayload turns on parsing of payload
	ParsePayload bool
}

// ParseLicensePEM parses license PEM, parses payload on demand
func ParseLicensePEM(pem []byte, opts ParseOptions) (*License, error) {
	certPEM, keyPEM, err := SplitPEM([]byte(pem))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certificateBytes, privateBytes, err := parseCertificatePEM(string(pem))
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
	if !opts.ParsePayload {
		return &license, nil
	}
	payload, err := unmarshalPayload(rawPayload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// decrypt encryption key
	if len(payload.EncryptionKey) != 0 {
		private, err := x509.ParsePKCS1PrivateKey(privateBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		payload.EncryptionKey, err = rsa.DecryptOAEP(sha256.New(), rand.Reader,
			private, payload.EncryptionKey, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	license.Payload = *payload
	return &license, nil
}

// ParseX509 parses the license from the provided x509 certificate
func ParseX509(cert *x509.Certificate) (*License, error) {
	payload, err := parsePayloadFromX509(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &License{
		Cert:    cert,
		Payload: *payload,
	}, nil
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

// unmarshalPayload unmarshals payload encoded in license
func unmarshalPayload(raw []byte) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

// parsePayloadFromX509 parses the extension with license payload from the
// provided x509 certificate
func parsePayloadFromX509(cert *x509.Certificate) (*Payload, error) {
	raw, err := getRawPayloadFromX509(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return unmarshalPayload(raw)
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
