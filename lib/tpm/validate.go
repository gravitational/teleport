/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tpm

import (
	"context"
	"crypto"
	"crypto/subtle"
	"crypto/x509"
	"log/slog"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
)

// ValidateParams are the parameters required to validate a TPM.
type ValidateParams struct {
	// EKCert should be the EK certificate in ASN.1 DER format. At least one of
	// EKCert or EKKey must be provided.
	EKCert []byte
	// EKKey should be the public part of the EK key in PKIX format. At least
	// one of EKCert or EKKey must be provided. If EKCert is provided, EKKey
	// will be ignored.
	EKKey []byte
	// AttestParams are the parameters required to attest the TPM provided by
	// the client. These relate to an AK that has been generated for this
	// ceremony.
	AttestParams attest.AttestationParameters
	// Solve is the function that Validate should call when it has prepared the
	// challenge and needs the remote TPM to solve it.
	Solve func(ec *attest.EncryptedCredential) ([]byte, error)
	// AllowedCAs is a pool of PEM encoded CAs that are allowed to sign the
	// EKCert. If this value is nil, the EKCert will not be verified.
	AllowedCAs *x509.CertPool
}

// ValidatedTPM is returned by Validate and contains the validated information
// about the remote TPM.
type ValidatedTPM struct {
	// EKPubHash is the SHA256 hash of the PKIX marshaled EKPub in hex format.
	EKPubHash string `json:"ek_pub_hash"`
	// EKCertSerial is the serial number of the EK cert represented as a colon
	// delimited hex string. If there is no EKCert, this field will be empty.
	EKCertSerial string `json:"ek_cert_serial,omitempty"`
	// EKCertVerified is true if the EKCert was verified against the allowed
	// CAs.
	EKCertVerified bool `json:"ek_cert_verified"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *ValidatedTPM) JoinAuditAttributes() (map[string]interface{}, error) {
	return map[string]interface{}{
		"ek_pub_hash":      c.EKPubHash,
		"ek_cert_serial":   c.EKCertSerial,
		"ek_cert_verified": c.EKCertVerified,
	}, nil
}

// Validate takes the parameters from a remote TPM and performs the necessary
// initial checks before then generating an encrypted credential challenge for
// the client to solve in a credential activation ceremony. This allows us to
// verify that the client possesses the TPM corresponding to the EK public key
// or certificate presented by the client.
func Validate(
	ctx context.Context, log *slog.Logger, params ValidateParams,
) (*ValidatedTPM, error) {
	ctx, span := tracer.Start(ctx, "Validate")
	defer span.End()

	// Validate params
	switch {
	case params.Solve == nil:
		return nil, trace.BadParameter("solve must be non-nil")
	case params.EKCert == nil && params.EKKey == nil:
		return nil, trace.BadParameter("at least one of EKCert or EKKey must be provided")
	}

	ekCert, ekPub, err := parseEK(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validated := &ValidatedTPM{}
	ekPubPKIX, err := x509.MarshalPKIXPublicKey(ekPub)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	validated.EKPubHash, err = hashEKPub(ekPubPKIX)
	if err != nil {
		return validated, trace.Wrap(err, "hashing EK public key")
	}
	if ekCert != nil {
		validated.EKCertSerial = serialString(ekCert.SerialNumber)
	}

	if params.AllowedCAs != nil {
		if err := verifyEKCert(ctx, params.AllowedCAs, ekCert); err != nil {
			return validated, trace.Wrap(err, "verifying EK cert")
		}
		validated.EKCertVerified = true
	}

	activationParameters := attest.ActivationParameters{
		TPMVersion: attest.TPMVersion20,
		AK:         params.AttestParams,
		EK:         ekPub,
	}
	// The generate method completes initial validation that provides the
	// following assurances:
	// - The attestation key is of a secure length
	// - The attestation key is marked as created within a TPM
	// - The attestation key is marked as restricted (e.g cannot be used to
	//   sign or decrypt external data)
	// When the returned challenge is solved by the TPM using ActivateCredential
	// the following additional assurance is given:
	// - The attestation key resides in the same TPM as the endorsement key
	solution, encryptedCredential, err := activationParameters.Generate()
	if err != nil {
		return validated, trace.Wrap(err, "generating credential activation challenge")
	}
	clientSolution, err := params.Solve(encryptedCredential)
	if err != nil {
		return validated, trace.Wrap(err, "asking client to perform credential activation")
	}
	if subtle.ConstantTimeCompare(clientSolution, solution) != 1 {
		return validated, trace.BadParameter("invalid credential activation solution")
	}

	return validated, nil
}

func parseEK(
	ctx context.Context, params ValidateParams,
) (*x509.Certificate, crypto.PublicKey, error) {
	_, span := tracer.Start(ctx, "parseEK")
	defer span.End()

	ekCertPresent := len(params.EKCert) > 0
	ekKeyPresent := len(params.EKKey) > 0
	switch {
	case ekCertPresent:
		ekCert, err := attest.ParseEKCertificate(params.EKCert)
		if err != nil {
			return nil, nil, trace.Wrap(err, "parsing EK cert")
		}
		return ekCert, ekCert.PublicKey, nil
	case ekKeyPresent:
		ekPub, err := x509.ParsePKIXPublicKey(params.EKKey)
		if err != nil {
			return nil, nil, trace.Wrap(err, "parsing EK key")
		}
		return nil, ekPub, nil
	default:
		return nil, nil, trace.BadParameter("either EK cert or EK key must be provided")
	}
}

func verifyEKCert(
	ctx context.Context,
	allowedCAs *x509.CertPool,
	ekCert *x509.Certificate,
) error {
	_, span := tracer.Start(ctx, "verifyEKCert")
	defer span.End()

	if ekCert == nil {
		return trace.BadParameter("tpm did not provide an EKCert to validate against allowed CAs")
	}

	StripSANExtensionOIDs(ekCert)

	// Validate EKCert against CA pool
	_, err := ekCert.Verify(x509.VerifyOptions{
		Roots: allowedCAs,
		KeyUsages: []x509.ExtKeyUsage{
			// Go's x509 Verification doesn't support the EK certificate
			// ExtKeyUsage (http://oid-info.com/get/2.23.133.8.1), so we
			// allow any.
			x509.ExtKeyUsageAny,
		},
	})
	if err != nil {
		return trace.Wrap(err, "verifying EK cert")
	}
	return nil
}

var sanExtensionOID = []int{2, 5, 29, 17}

// StripSANExtensionOIDs removes the SAN Extension OID from the specified
// cert. This method may re-assign the remaining extensions out of order.
//
// This is necessary because the EKCert may contain additional data
// bundled within the SAN extension. This ext is also sometimes marked
// critical. This causes the Verify() to reject the cert because not all data
// within a critical extension has been handled. We mark this as OK here by
// stripping the SAN Extension OID out of UnhandledCriticalExtensions.
func StripSANExtensionOIDs(cert *x509.Certificate) {
	for i := 0; i < len(cert.UnhandledCriticalExtensions); i++ {
		ext := cert.UnhandledCriticalExtensions[i]
		if !ext.Equal(sanExtensionOID) {
			continue
		}
		// Swap ext with the last index and remove it.
		last := len(cert.UnhandledCriticalExtensions) - 1
		cert.UnhandledCriticalExtensions[i] = cert.UnhandledCriticalExtensions[last]
		cert.UnhandledCriticalExtensions[last] = nil // "Release" extension
		cert.UnhandledCriticalExtensions = cert.UnhandledCriticalExtensions[:last]
		i--
	}
}
