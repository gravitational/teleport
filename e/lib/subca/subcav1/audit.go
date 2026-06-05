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
	"crypto/x509"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/subca"
)

func (s *Service) emitCAOverrideEvent(
	ctx context.Context,
	parsed *subca.ParsedCertAuthorityOverride,
	err error,
	eventType, eventCode string,
) {
	// Sanity check input.
	// This is a private helper, so we can ensure all callers pass a non-nil
	// parsed override.
	if parsed == nil ||
		parsed.CAOverride == nil ||
		parsed.CAOverride.GetSubKind() == "" ||
		parsed.CAOverride.GetMetadata().GetName() == "" {
		s.logger.ErrorContext(ctx,
			"CA override required to issue audit event",
			"error", trace.BadParameter("parsed CA override required"), // capture trace
			"event_type", eventType,
			"event_code", eventCode,
			"parsed_ca_override", parsed,
		)
		return
	}

	um := authz.ClientUserMetadata(ctx)

	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}

	auditName := parsed.CAOverride.GetSubKind() + "/" + parsed.CAOverride.GetMetadata().GetName()

	e := &apievents.CertAuthorityOverrideEvent{
		Metadata: apievents.Metadata{
			Type: eventType,
			Code: eventCode,
		},
		UserMetadata: um,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      auditName,
			UpdatedBy: um.GetUser(),
		},
		Status: apievents.Status{
			Success: err == nil,
			Error:   errorMessage,
		},
		CaOverride: caOverrideToEventMetadata(parsed),
	}

	if err := s.emitter.EmitAuditEvent(ctx, e); err != nil {
		s.logger.WarnContext(ctx,
			"Failed to emit audit event",
			"error", err,
			"type", e.GetType(),
			"code", e.GetCode(),
			"user", um.User,
			"impersonator", um.Impersonator,
		)
	}
}

func caOverrideToEventMetadata(parsed *subca.ParsedCertAuthorityOverride) *apievents.CertAuthorityOverrideMetadata {
	overrides := make([]*apievents.CertificateOverrideMetadata, len(parsed.CertificateOverrides))
	for i, co := range parsed.CertificateOverrides {
		overrideMeta := &apievents.CertificateOverrideMetadata{
			Certificate: certificateToEventMetadata(co.PublicKey, co.Certificate),
			Disabled:    co.CertificateOverride.GetDisabled(),
		}
		for _, chainCert := range co.Chain {
			overrideMeta.Chain = append(
				overrideMeta.Chain,
				certificateToEventMetadata("", chainCert),
			)
		}
		overrides[i] = overrideMeta
	}

	return &apievents.CertAuthorityOverrideMetadata{
		CaType:               parsed.CAOverride.GetSubKind(),
		ClusterName:          parsed.CAOverride.GetMetadata().GetName(),
		CertificateOverrides: overrides,
	}
}

func certificateToEventMetadata(
	publicKeyHash string,
	cert *x509.Certificate,
) *apievents.X509OverrideMetadata {
	if cert == nil {
		return nil
	}

	return &apievents.X509OverrideMetadata{
		Issuer:        cert.Issuer.String(),
		Subject:       cert.Subject.String(),
		SerialNumber:  cert.SerialNumber.String(),
		PublicKeyHash: publicKeyHash,
	}
}
