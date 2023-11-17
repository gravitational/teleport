// Copyright 2023 Gravitational, Inc
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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/externalcloudaudit"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// GenerateExternalCloudAuditOIDCToken generates a signed OIDC token for use by
// the ExternalCloudAudit feature when authenticating to customer AWS accounts.
func (a *Server) GenerateExternalCloudAuditOIDCToken(ctx context.Context) (string, error) {
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
		Expires:  a.clock.Now().Add(externalcloudaudit.TokenLifetime),
	})
	return token, trace.Wrap(err)
}
