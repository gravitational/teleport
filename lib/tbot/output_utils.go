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

package tbot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
)

// concatCACerts borrow's identityfile's CA cert concat method.
func concatCACerts(cas []types.CertAuthority) []byte {
	trusted := authclient.AuthoritiesToTrustedCerts(cas)

	var caCerts []byte
	for _, ca := range trusted {
		for _, cert := range ca.TLSCertificates {
			caCerts = append(caCerts, cert...)
		}
	}

	return caCerts
}

// describeTLSIdentity generates an informational message about the given
// TLS identity, appropriate for user-facing log messages.
func describeTLSIdentity(ctx context.Context, log *slog.Logger, ident *identity.Identity) string {
	failedToDescribe := "failed-to-describe"
	cert := ident.X509Cert
	if cert == nil {
		log.WarnContext(ctx, "Attempted to describe TLS identity without TLS credentials.")
		return failedToDescribe
	}

	tlsIdent, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		log.WarnContext(ctx, "Bot TLS certificate can not be parsed as an identity", "error", err)
		return failedToDescribe
	}

	var principals []string
	for _, principal := range tlsIdent.Principals {
		if !strings.HasPrefix(principal, constants.NoLoginPrefix) {
			principals = append(principals, principal)
		}
	}

	botDesc := ""
	if tlsIdent.BotInstanceID != "" {
		botDesc = fmt.Sprintf(", id=%s", tlsIdent.BotInstanceID)
	}

	duration := cert.NotAfter.Sub(cert.NotBefore)
	return fmt.Sprintf(
		"%s%s | valid: after=%v, before=%v, duration=%s | kind=tls, renewable=%v, disallow-reissue=%v, roles=%v, principals=%v, generation=%v",
		tlsIdent.BotName,
		botDesc,
		cert.NotBefore.Format(time.RFC3339),
		cert.NotAfter.Format(time.RFC3339),
		duration,
		tlsIdent.Renewable,
		tlsIdent.DisallowReissue,
		tlsIdent.Groups,
		principals,
		tlsIdent.Generation,
	)
}
