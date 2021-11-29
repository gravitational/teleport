// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webauthn

import (
	"crypto/x509"
	"encoding/pem"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// x5cFormats enumerates all attestation formats that supply an attestation
// chain through the "x5c" field.
// See https://www.w3.org/TR/webauthn/#sctn-defined-attestation-formats.
var x5cFormats = []string{
	"packed",
	"tpm",
	"android-key",
	"fido-u2f",
	"apple",
}

func verifyAttestation(cfg *types.Webauthn, obj protocol.AttestationObject) error {
	if len(cfg.AttestationAllowedCAs) == 0 && len(cfg.AttestationDeniedCAs) == 0 {
		return nil // Attestation disabled.
	}

	attestationChain, err := getChainFromObj(obj)
	if err != nil {
		return trace.Wrap(
			err, "failed to read attestation certificate; make sure you are using a device from a trusted manufacturer")
	}

	// We don't really expect errors at this stage, by the time the configuration
	// gets here it was already validated by Teleport.
	allowedPool, err := x509PEMsToCertPool(cfg.AttestationAllowedCAs)
	if err != nil {
		return trace.Wrap(err, "invalid webauthn attestation_allowed_ca")
	}
	deniedPool, err := x509PEMsToCertPool(cfg.AttestationDeniedCAs)
	if err != nil {
		return trace.Wrap(err, "invalid webauthn attestation_denied_ca")
	}

	// Attestation check works as follows:
	// 1. At least one certificate must belong to the allowed pool.
	// 2. No certificates may belong to the denied pool.
	//
	// It is possible for both allowed and denied CAs to be present. It's also
	// possible for configurations to allow a broad range of options (eg, all
	// YubiKey devices) while denying a smaller subset (a certain model or lot),
	// so both checks (allowed and denied) may be true for the same cert.
	allowed := len(cfg.AttestationAllowedCAs) == 0
	for _, cert := range attestationChain {
		if _, err := cert.Verify(x509.VerifyOptions{Roots: allowedPool}); err == nil {
			allowed = true // OK, but keep checking
		}
		if _, err := cert.Verify(x509.VerifyOptions{Roots: deniedPool}); err == nil {
			return trace.BadParameter("attestation certificate %q from issuer %q not allowed", cert.Subject, cert.Issuer)
		}
	}
	if !allowed {
		return trace.BadParameter(
			"failed to verify device attestation certificate; make sure you are using a device from a trusted manufacturer")
	}
	return nil
}

func x509PEMsToCertPool(certPEMs []string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, cert := range certPEMs {
		if !pool.AppendCertsFromPEM([]byte(cert)) {
			return nil, trace.BadParameter("failed to parse certificate PEM")
		}
	}
	return pool, nil
}

func getChainFromObj(obj protocol.AttestationObject) ([]*x509.Certificate, error) {
	if utils.SliceContainsStr(x5cFormats, obj.Format) {
		return getChainFromX5C(obj)
	}
	if obj.Format == "none" {
		// Return a nicer error for "none", since we do allow it in non-attestation
		// scenarios.
		return nil, trace.BadParameter("attestation format %q not allowed for direct attestation", obj.Format)
	}
	return nil, trace.BadParameter("attestation format %q not supported", obj.Format)
}

func getChainFromX5C(obj protocol.AttestationObject) ([]*x509.Certificate, error) {
	x5c, ok := obj.AttStatement["x5c"]
	if !ok {
		// Warn about self-attestation and Touch ID, it may save someone some grief.
		return nil, trace.BadParameter(
			"%q attestation: self attestation not allowed; includes Touch ID in non-Apple browsers", obj.Format)
	}
	x5cArray, ok := x5c.([]interface{})
	if !ok {
		return nil, trace.BadParameter("%q attestation: unexpected x5c type: %T", obj.Format, x5c)
	}
	if len(x5cArray) == 0 {
		return nil, trace.BadParameter("%q attestation: empty certificate chain", obj.Format)
	}
	chain := make([]*x509.Certificate, len(x5cArray))
	for i, val := range x5cArray {
		cert, ok := val.([]byte)
		if !ok {
			return nil, trace.BadParameter("%q attestation: unexpected x5c element type at index %v: %T", obj.Format, i, val)
		}
		var err error
		chain[i], err = x509.ParseCertificate(cert)
		if err != nil {
			return nil, trace.Wrap(err, "%q attestation: failed to parse certificate at index %v", obj.Format, i)
		}
	}

	// Print out attestation certs if debug is enabled.
	// This may come in handy for people having trouble with their setups.
	if log.IsLevelEnabled(log.DebugLevel) {
		for _, cert := range chain {
			certPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: cert.Raw,
			})
			log.Debugf("WebAuthn: got %q attestation certificate:\n\n%s", obj.Format, certPEM)
		}
	}

	return chain, nil
}
