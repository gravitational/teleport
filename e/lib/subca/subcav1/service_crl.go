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

package subcav1

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/subca"
)

// createOverrideCRLs creates CRLs for all overrides that lack them.
// Updates the parsed.CAOverride.Status field.
func (s *Service) createOverrideCRLs(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	parsed *subca.ParsedCertAuthorityOverride,
) error {
	now := s.clock.Now()
	data := findCRLsToGenerate(ctx, parsed, now)

	// Trim Status to the necessary CRLs.
	status := parsed.CAOverride.GetStatus()
	for k := range status.GetPublicKeyHashToCrl() {
		if _, seen := data.SeenCertPublicKeyHash[k]; !seen {
			delete(status.GetPublicKeyHashToCrl(), k)
		}
	}

	if len(data.NeedsCRL) == 0 {
		// Nothing to do.
		return nil
	}

	parsedCA, err := getParsedCA()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, co := range data.NeedsCRL {
		pkh := co.PublicKey

		// Find KeyPair.
		kp, ok := parsedCA.IndexedKeyPairs[pkh]
		if !ok {
			return trace.BadParameter("unknown CA certificate %q, cannot create CRL", pkh)
		}

		// Find already-parsed CA certificate.
		caCert, ok := parsedCA.ActiveKeyHashes[pkh]
		if !ok {
			caCert = parsedCA.AdditionalKeyHashes[pkh]
		}
		// Defensive. Should not happen.
		if caCert == nil {
			return trace.Wrap(fmt.Errorf("cannot find CA certificate %q in either active or additional keys", pkh))
		}

		// Prepare the CRL request.
		req := &x509.RevocationList{
			Number:     big.NewInt(1),             // Only one ever published.
			ThisUpdate: now.Add(-1 * time.Minute), // Allow for clock skew.
			NextUpdate: caCert.NotAfter,
		}

		// Attempt to get the signer for the KeyPair.
		// May fail on HSM-enabled, multi-Auth scenarios, as it needs access to a
		// specific Auth server then.
		signer, err := s.keystoreManager.TLSSigner(ctx, kp)
		switch {
		case errors.Is(err, keystore.ErrUnusableKey):
			return trace.BadParameter(
				"Auth server cannot sign for certificate %q, please connect to the appropriate Auth server to create this override",
				pkh,
			)
		case err != nil:
			return trace.Wrap(err, "create signer for override %q", pkh)
		}

		// Sign.
		crlDER, err := x509.CreateRevocationList(rand.Reader, req, co.Certificate, signer)
		if err != nil {
			return trace.Wrap(err, "create CRL for override %q", pkh)
		}
		s.logger.DebugContext(ctx,
			"Created CRL for override",
			"public_key_hash", pkh,
		)

		// Update status.
		crlPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "X509 CRL",
			Bytes: crlDER,
		})
		status.GetPublicKeyHashToCrl()[pkh] = subcav1.CertificateRevocationList_builder{
			Pem: string(crlPEM),
		}.Build()
	}

	return nil
}

type findCRLData struct {
	NeedsCRL              []*subca.ParsedCertificateOverride
	SeenCertPublicKeyHash map[string]struct{}
}

func findCRLsToGenerate(ctx context.Context, parsed *subca.ParsedCertAuthorityOverride, now time.Time) *findCRLData {
	data := &findCRLData{
		SeenCertPublicKeyHash: make(map[string]struct{}),
	}

	status := parsed.CAOverride.GetStatus()
	for _, co := range parsed.CertificateOverrides {
		if co.Certificate == nil {
			// Overrides without a certificate can't have a CRL.
			continue
		}
		pkh := co.PublicKey
		data.SeenCertPublicKeyHash[pkh] = struct{}{}

		crlPB, ok := status.GetPublicKeyHashToCrl()[pkh]
		if !ok {
			// CRL absent.
			data.NeedsCRL = append(data.NeedsCRL, co)
			continue
		}

		logger := slog.With(
			"ca_type", parsed.CAOverride.GetSubKind(),
			"cluster_name", parsed.CAOverride.GetMetadata().GetName(),
			"public_key_hash", pkh,
		)

		// Parse CRL.
		crl, err := parseRevocationListPB(crlPB)
		if err != nil {
			logger.DebugContext(ctx,
				"Failed to parse CA override CRL",
				"error", err,
			)
			data.NeedsCRL = append(data.NeedsCRL, co)
			continue
		}

		// Verify timestamps. This is more lenient than the CRL generation logic,
		// but sufficient to guarantee a valid CRL for the override.
		const thisUpdateGracePeriod = 2 * time.Minute
		if crl.ThisUpdate.After(now.Add(thisUpdateGracePeriod)) ||
			crl.NextUpdate.Before(co.Certificate.NotAfter) {
			logger.DebugContext(ctx,
				"CRL ThisUpdate or NextUpdate invalid",
				"crl_this_update", crl.ThisUpdate,
				"crl_next_update", crl.NextUpdate,
				"certificate_not_after", co.Certificate.NotAfter,
				"now", now,
			)
			data.NeedsCRL = append(data.NeedsCRL, co)
			continue
		}

		// Verify Issuer.
		if issuer, subject := crl.Issuer.String(), co.Certificate.Subject.String(); issuer != subject {
			logger.DebugContext(ctx,
				"CRL issuer does not match override certificate",
				"crl_issuer", issuer,
				"certificate_subject", subject,
			)
			data.NeedsCRL = append(data.NeedsCRL, co)
			continue
		}

		// Verify signature.
		if err := crl.CheckSignatureFrom(co.Certificate); err != nil {
			logger.DebugContext(ctx,
				"CRL signature verification failed",
				"error", err,
			)
			data.NeedsCRL = append(data.NeedsCRL, co)
			continue
		}

		// CRL is valid. No need to regenerate.
	}
	return data
}

func parseRevocationListPB(crlPB *subcav1.CertificateRevocationList) (*x509.RevocationList, error) {
	if len(crlPB.GetPem()) == 0 {
		return nil, trace.BadParameter("empty or nil CRL PB")
	}
	block, _ := pem.Decode([]byte(crlPB.GetPem()))
	if block == nil {
		return nil, trace.BadParameter("failed to parse CRL PEM")
	}
	crl, err := x509.ParseRevocationList(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err, "parse CRL")
	}
	return crl, nil
}
