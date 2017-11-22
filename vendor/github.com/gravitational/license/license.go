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
	"time"

	"github.com/gravitational/trace"
)

// License represents Gravitational license
type License struct {
	// Cert is the x509 license certificate
	Cert *x509.Certificate
	// Payload is the license payload
	Payload Payload
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

// Payload is custom information that gets encoded into licenses
type Payload struct {
	// ClusterID is vendor-specific cluster ID
	ClusterID string `json:"cluster_id,omitempty"`
	// Expiration is expiration time for the license
	Expiration time.Time `json:"expiration,omitempty"`
	// MaxNodes is maximum number of nodes the license allows
	MaxNodes int `json:"max_nodes,omitempty"`
	// MaxCores is maximum number of CPUs per node the license allows
	MaxCores int `json:"max_cores,omitempty"`
	// Company is the company name the license is generated for
	Company string `json:"company,omitempty"`
	// Person is the name of the person the license is generated for
	Person string `json:"person,omitempty"`
	// Email is the email of the person the license is generated for
	Email string `json:"email,omitempty"`
	// Metadata is an arbitrary customer metadata
	Metadata string `json:"metadata,omitempty"`
	// ProductName is the name of the product the license is for
	ProductName string `json:"product_name,omitempty"`
	// ProductVersion is the product version
	ProductVersion string `json:"product_version,omitempty"`
	// EncryptionKey is the passphrase for decoding encrypted packages
	EncryptionKey []byte `json:"encryption_key,omitempty"`
	// Signature is vendor-specific signature
	Signature string `json:"signature,omitempty"`
	// Shutdown indicates whether the app should be stopped when the license expires
	Shutdown bool `json:"shutdown,omitempty"`
	// AccountID is the ID of the account the license was issued for
	AccountID string `json:"account_id,omitempty"`
}
