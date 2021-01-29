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
	"crypto/x509"

	"github.com/gravitational/trace"
)

// License represents Gravitational license
type License struct {
	// Cert is the x509 license certificate
	Cert *x509.Certificate
	// RawPayload contains raw license payload
	RawPayload []byte
	// CertPEM is the certificate part of the license in PEM format
	CertPEM []byte
	// KeyPEM is the private key part of the license in PEM Format,
	// may be empty if the license was parsed from certificate only
	KeyPEM []byte
}

// Verify makes sure the license is valid
func (l *License) Verify(caPEM []byte) error {
	roots := x509.NewCertPool()

	// add the provided CA certificate to the roots
	ok := roots.AppendCertsFromPEM(caPEM)
	if !ok {
		return trace.BadParameter("could not find any CA certificates")
	}

	_, err := l.Cert.Verify(x509.VerifyOptions{Roots: roots})
	if err != nil {
		certErr, ok := err.(x509.CertificateInvalidError)
		if ok && certErr.Reason == x509.Expired {
			return trace.BadParameter("the license has expired")
		}
		return trace.Wrap(err, "failed to verify the license")
	}

	return nil
}
