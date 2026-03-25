// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package tpmjoin

import (
	"context"
	"crypto/x509"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tpm"
)

// TPMValidator is a function type that validates a TPM for the TPM join method.
type TPMValidator func(ctx context.Context, params tpm.ValidateParams) (*tpm.ValidatedTPM, error)

// CheckTPMRequestParams holds all parameters for CheckTPMRequest.
type CheckTPMRequestParams struct {
	// Token is the provision token used to validate the request.
	Token *types.ProvisionTokenV2
	// TPMValidator is a function that will be called to validate the presented TPM.
	TPMValidator TPMValidator

	// EKCert is the device's endorsement certificate in X509, ASN.1 DER form.
	// This certificate contains the public key of the endorsement key. This is
	// preferred to ek_key.
	EKCert []byte
	// The device's public endorsement key in PKIX, ASN.1 DER form. This is
	// used when a TPM does not contain any endorsement certificates.
	EKKey []byte
	// AttestationParameters describes information about a key which is necessary
	// for verifying its properties remotely.
	AttestParams attest.AttestationParameters
	// Solve is the function will be called when TPMValidator has prepared the
	// challenge and needs the remote TPM to solve it.
	Solve func(*attest.EncryptedCredential) ([]byte, error)
}

// CheckTPMRequest checks a TPM method join request.
func CheckTPMRequest(ctx context.Context, params CheckTPMRequestParams) (*tpm.ValidatedTPM, error) {
	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.Wrap(
			services.ErrRequiresEnterprise,
			"tpm joining",
		)
	}

	certPool, err := buildCertPool(params.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validatedEK, err := params.TPMValidator(ctx, tpm.ValidateParams{
		EKCert:       params.EKCert,
		EKKey:        params.EKKey,
		AttestParams: params.AttestParams,
		AllowedCAs:   certPool,
		Solve:        params.Solve,
	})
	if err != nil {
		return nil, trace.AccessDenied("validating TPM: %v", err)
	}

	if err := checkTPMAllowRules(validatedEK, params.Token.Spec.TPM.Allow); err != nil {
		return validatedEK, trace.Wrap(err)
	}

	return validatedEK, nil
}

func buildCertPool(token *types.ProvisionTokenV2) (*x509.CertPool, error) {
	if len(token.Spec.TPM.EKCertAllowedCAs) == 0 {
		// Certs are not validated if no CAs were configured.
		return nil, nil
	}
	certPool := x509.NewCertPool()
	for i, ca := range token.Spec.TPM.EKCertAllowedCAs {
		if ok := certPool.AppendCertsFromPEM([]byte(ca)); !ok {
			return nil, trace.BadParameter(
				"ekcert_allowed_cas[%d] has an invalid or malformed PEM", i,
			)
		}
	}
	return certPool, nil
}

func checkTPMAllowRules(tpm *tpm.ValidatedTPM, rules []*types.ProvisionTokenSpecV2TPM_Rule) error {
	// If a single rule passes, accept the TPM
	for _, rule := range rules {
		if rule.EKPublicHash != "" && tpm.EKPubHash != rule.EKPublicHash {
			continue
		}
		if rule.EKCertificateSerial != "" && tpm.EKCertSerial != rule.EKCertificateSerial {
			continue
		}

		// All rules met.
		return nil
	}
	return trace.AccessDenied("validated tpm attributes did not match any allow rules")
}
