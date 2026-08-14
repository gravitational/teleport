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
	"cmp"
	"context"
	"crypto/x509"
	"log/slog"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/subca"
)

type auditWriter struct {
	logger  *slog.Logger
	emitter apievents.Emitter

	eventType, eventCode string
	caOverride           *subcav1.CertAuthorityOverride
	parsed               *subca.ParsedCertAuthorityOverride
}

func (s *Service) newAuditWriter(
	eventType string,
	eventCode string,
	caOverride *subcav1.CertAuthorityOverride,
) *auditWriter {
	return &auditWriter{
		logger:     s.logger,
		emitter:    s.emitter,
		eventType:  eventType,
		eventCode:  eventCode,
		caOverride: caOverride,
	}
}

func (w *auditWriter) setParsed(parsed *subca.ParsedCertAuthorityOverride) {
	w.caOverride = nil
	w.parsed = parsed
}

func (w *auditWriter) emitAuditEvent(ctx context.Context, err error) {
	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}

	switch {
	case w.parsed == nil && w.caOverride == nil:
		w.logger.DebugContext(ctx,
			"Audit event lacks both parsed and non-parsed CA override. Continuing with minimal data.",
		)
		w.parsed = &subca.ParsedCertAuthorityOverride{
			CAOverride: &subcav1.CertAuthorityOverride{},
		}
	case w.parsed == nil:
		var err error
		w.parsed, err = subca.ParseCAOverride(w.caOverride)
		if err != nil {
			w.logger.DebugContext(ctx,
				"Failed to parse CA override for audit event. Continuing with minimal data.",
				"error", err,
			)
			w.parsed = &subca.ParsedCertAuthorityOverride{
				CAOverride: w.caOverride,
			}
		}
	}
	parsed := w.parsed

	var auditName string
	{
		caType := cmp.Or(parsed.CAOverride.GetSubKind(), "<unknown>")
		clusterName := cmp.Or(parsed.CAOverride.GetMetadata().GetName(), "<unknown>")
		auditName = caType + "/" + clusterName
	}

	um := authz.ClientUserMetadata(ctx)

	e := &apievents.CertAuthorityOverrideEvent{
		Metadata: apievents.Metadata{
			Type: w.eventType,
			Code: w.eventCode,
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

	if err := w.emitter.EmitAuditEvent(ctx, e); err != nil {
		w.logger.WarnContext(ctx,
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
