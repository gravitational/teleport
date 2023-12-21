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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// GenerateExternalAuditStorageOIDCToken generates a signed OIDC token for use by
// the External Audit Storage feature when authenticating to customer AWS accounts.
func (a *Server) GenerateExternalAuditStorageOIDCToken(ctx context.Context) (string, error) {
	clusterName, err := a.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}

	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName,
	}, true /*loadKeys*/)
	if err != nil {
		return "", trace.Wrap(err)
	}

	signer, err := a.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), a.clock)
	if err != nil {
		return "", trace.Wrap(err)
	}

	issuer, err := oidc.IssuerForCluster(ctx, a)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := privateKey.SignAWSOIDC(jwt.SignParams{
		Username: a.ServerID,
		Audience: types.IntegrationAWSOIDCAudience,
		Subject:  types.IntegrationAWSOIDCSubjectAuth,
		Issuer:   issuer,
		Expires:  a.clock.Now().Add(externalauditstorage.TokenLifetime),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ExternalAuditStorageAuthenticateEvent{})

	return token, nil
}
