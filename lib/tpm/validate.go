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
	"github.com/mitchellh/mapstructure"
)

type ValidateParams struct {
	EKCert       []byte
	EKKey        []byte
	AttestParams attest.AttestationParameters
	Solve        func(ec *attest.EncryptedCredential) ([]byte, error)
	AllowedCAs   []string
}

type ValidatedTPM struct {
	EKPubHash      string `json:"ek_pub_hash"`
	EKCertSerial   string `json:"ek_cert_serial,omitempty"`
	EKCertVerified bool   `json:"ek_cert_verified"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *ValidatedTPM) JoinAuditAttributes() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}

func Validate(
	ctx context.Context, log *slog.Logger, params ValidateParams,
) (*ValidatedTPM, error) {
	ekCert, ekPub, err := parseEK(ctx, log, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validated := &ValidatedTPM{}
	validated.EKPubHash, err = hashEKPub(ekPub)
	if err != nil {
		return validated, trace.Wrap(err, "hashing EK public key")
	}
	if ekCert != nil {
		validated.EKCertSerial = serialString(ekCert.SerialNumber)
	}

	if len(params.AllowedCAs) > 0 {
		if err := verifyEKCert(ctx, log, params.AllowedCAs, ekCert); err != nil {
			log.ErrorContext(
				ctx,
				"EKCert CA verification failed",
				"error", err,
				"tpm", validated,
			)
			return validated, trace.Wrap(err)
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
	if subtle.ConstantTimeCompare(clientSolution, solution) == 0 {
		log.ErrorContext(
			ctx,
			"TPM Credential Activation solution did not match expected solution.",
			"error", err,
			"tpm", validated,
		)
		return validated, trace.BadParameter("invalid credential activation solution")
	}

	return validated, nil
}

func parseEK(
	ctx context.Context, log *slog.Logger, params ValidateParams,
) (*x509.Certificate, crypto.PublicKey, error) {
	ekCertPresent := len(params.EKCert) > 0
	ekKeyPresent := len(params.EKKey) > 0
	switch {
	case ekCertPresent:
		ekCert, err := attest.ParseEKCertificate(params.EKCert)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ekPub, err := x509.MarshalPKIXPublicKey(ekCert.PublicKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return ekCert, ekPub, nil
	case ekKeyPresent:
		ekPub, err := x509.ParsePKIXPublicKey(params.EKKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return nil, ekPub, nil
	default:
		return nil, nil, trace.BadParameter("either EK cert or EK key must be provided")
	}
}

func verifyEKCert(
	ctx context.Context,
	log *slog.Logger,
	allowedCAs []string,
	ekCert *x509.Certificate,
) error {
	if ekCert == nil {
		return trace.BadParameter("tpm did not provide an EKCert to validate against allowed CAs")
	}

	// Collect CAs into a pool to use for validation
	caPool := x509.NewCertPool()
	for _, caPEM := range allowedCAs {
		if !caPool.AppendCertsFromPEM([]byte(caPEM)) {
			return trace.BadParameter("invalid CA PEM")
		}
	}
	// Validate EKCert against CA pool
	_, err := ekCert.Verify(x509.VerifyOptions{
		Roots: caPool,
		KeyUsages: []x509.ExtKeyUsage{
			// Go's x509 Verification doesn't support the EK certificate
			// ExtKeyUsage (http://oid-info.com/get/2.23.133.8.1), so we
			// allow any.
			x509.ExtKeyUsageAny,
		},
	})
	if err != nil {
		return trace.Wrap(err, "verifying ekcert")
	}
	return nil
}
