// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testenv

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
)

var caSerialNumberGenerator atomic.Int64

// CAParams holds creation parameters for [CA].
type CAParams struct {
	// Clock used as a basis for NotBefore/NotAfter timestamps.
	// Optional. If unset a real clock is used.
	Clock clockwork.Clock
	// Pub is the certificate public key.
	// If present a key won't be created for the new CA, so the parent CA must
	// be supplied.
	Pub crypto.PublicKey
	// Template for the CA certificate.
	// Optional.
	Template *x509.Certificate
}

// CA represents a CA external to Teleport.
type CA struct {
	Key     *ecdsa.PrivateKey
	Cert    *x509.Certificate
	CertPEM []byte
}

// NewSelfSignedCA creates a new self-signed, "external" CA for testing.
func NewSelfSignedCA(optionalParams *CAParams) (*CA, error) {
	return createCA(optionalParams, nil /* parent */)
}

// NewIntermediateCA creates a new intermediate CA from the current CA.
func (ca *CA) NewIntermediateCA(optionalParams *CAParams) (*CA, error) {
	return createCA(optionalParams, ca)
}

func createCA(optionalParams *CAParams, parent *CA) (*CA, error) {
	var clock clockwork.Clock
	var pub crypto.PublicKey
	var template *x509.Certificate
	if optionalParams != nil {
		clock = optionalParams.Clock
		pub = optionalParams.Pub
		if optionalParams.Template != nil {
			cp := *optionalParams.Template
			template = &cp
		}
	}
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	if template == nil {
		template = &x509.Certificate{}
	}

	var key *ecdsa.PrivateKey
	if pub == nil {
		var err error
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}
		pub = key.Public()
	} else if parent == nil {
		// Guard against us not having a private key to sign with.
		return nil, errors.New("parent CA must be non-nil if Pub is set")
	}

	var issuerName *pkix.Name
	var issuerKey *ecdsa.PrivateKey
	var issuerCert *x509.Certificate
	if parent == nil {
		issuerKey = key
		issuerCert = template
	} else {
		issuerName = &parent.Cert.Subject
		issuerKey = parent.Key
		issuerCert = parent.Cert
	}

	prepareCATemplate(template, clock, issuerName)

	certRaw, err := x509.CreateCertificate(rand.Reader, template, issuerCert, pub, issuerKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certRaw)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	return &CA{
		Key:     key,
		Cert:    cert,
		CertPEM: certPEM,
	}, nil
}

func prepareCATemplate(dst *x509.Certificate, clock clockwork.Clock, issuer *pkix.Name) {
	// SerialNumber.
	if dst.SerialNumber == nil {
		sn := caSerialNumberGenerator.Add(1)
		dst.SerialNumber = big.NewInt(sn)
	}

	// Subject: CN and SerialNumber.
	if dst.Subject.CommonName == "" {
		if issuer == nil {
			dst.Subject.CommonName = fmt.Sprintf("EXTERNAL ROOT CA %s", dst.SerialNumber)
		} else {
			dst.Subject.CommonName = fmt.Sprintf("EXTERNAL INTERMEDIATE CA %s", dst.SerialNumber)
		}
	}
	if dst.Subject.SerialNumber == "" {
		dst.Subject.SerialNumber = dst.SerialNumber.String()
	}

	// Issuer.
	if issuer == nil {
		dst.Issuer = dst.Subject
	} else {
		dst.Issuer = *issuer
	}

	// NotBefore and NotAfter.
	now := clock.Now()
	if dst.NotBefore.IsZero() {
		dst.NotBefore = now.Add(-1 * time.Minute)
	}
	if dst.NotAfter.IsZero() {
		dst.NotAfter = now.Add(1 * time.Hour)
	}

	// Usage, constraints, etc.
	// Take an opinionated stance on the fields below.
	dst.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	dst.ExtKeyUsage = nil
	dst.BasicConstraintsValid = true
	dst.IsCA = true
}

// CAChain is a chain of external CAs, root-to-intermediates.
type CAChain []*CA

// MakeCAChain creates an external CA certificate chain.
//
// The certificate template in optionalParams is ignored.
func MakeCAChain(length int, optionalParams *CAParams) (CAChain, error) {
	var params *CAParams
	if optionalParams != nil {
		cp := *optionalParams
		params = &cp
	} else {
		params = &CAParams{}
	}
	params.Template = &x509.Certificate{}

	cas := make([]*CA, 0, length)
	for i := range length {
		params.Template.MaxPathLen = length - i - 1

		if i == 0 {
			ca, err := NewSelfSignedCA(params)
			if err != nil {
				return nil, fmt.Errorf("create root CA: %w", err)
			}
			cas = append(cas, ca)
			continue
		}

		parent := cas[i-1]
		ca, err := parent.NewIntermediateCA(params)
		if err != nil {
			return nil, fmt.Errorf("create intermediate CA: %w", err)
		}
		cas = append(cas, ca)
	}

	return cas, nil
}

// RootToLeafPEMs returns the chain certificates, in PEM form, in root-to-leaves
// order.
func (chain CAChain) RootToLeafPEMs() []string {
	pems := make([]string, len(chain))
	for i, ca := range chain {
		pems[i] = string(ca.CertPEM)
	}
	return pems
}

// LeafToRootPEMs returns the chain certificates, in PEM form, in leaves-to-root
// order.
func (chain CAChain) LeafToRootPEMs() []string {
	pems := chain.RootToLeafPEMs()
	slices.Reverse(pems)
	return pems
}
