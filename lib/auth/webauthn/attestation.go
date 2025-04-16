/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package webauthn

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"slices"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "WebAuthn")

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

	verifyOptsBase := x509.VerifyOptions{
		// TPM-bound certificates, like those issued for Windows Hello, set
		// ExtKeyUsage OID 2.23.133.8.3, aka "AIK (Attestation Identity Key)
		// certificate".
		//
		// There isn't an ExtKeyUsage constant for that, so we allow any.
		//
		// - https://learn.microsoft.com/en-us/windows/apps/develop/security/windows-hello#attestation
		// - https://oid-base.com/get/2.23.133.8.3
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
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
		opts := verifyOptsBase // take copy
		opts.Roots = allowedPool
		if _, err := cert.Verify(opts); err == nil {
			allowed = true // OK, but keep checking
		} else {
			log.DebugContext(context.Background(),
				"Attestation check for allowed CAs failed",
				"subject", cert.Subject,
				"error", err,
			)
		}

		opts = verifyOptsBase // take copy
		opts.Roots = deniedPool
		if _, err := cert.Verify(opts); err == nil {
			return trace.BadParameter("attestation certificate %q from issuer %q not allowed", cert.Subject, cert.Issuer)
		} else if !errors.As(err, new(x509.UnknownAuthorityError)) {
			log.DebugContext(context.Background(),
				"Attestation check for denied CAs failed",
				"subject", cert.Subject,
				"error", err,
			)
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
	if slices.Contains(x5cFormats, obj.Format) {
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
	ctx := context.Background()
	if log.Handler().Enabled(ctx, slog.LevelDebug) {
		for _, cert := range chain {
			certPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: cert.Raw,
			})
			log.DebugContext(context.Background(), "got attestation certificate",
				"format", obj.Format,
				"certificate", string(certPEM),
			)
		}
	}

	return chain, nil
}
