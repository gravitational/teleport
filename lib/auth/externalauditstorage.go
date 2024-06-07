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
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// GenerateExternalAuditStorageOIDCToken generates a signed OIDC token for use by
// the External Audit Storage feature when authenticating to customer AWS accounts.
func (a *Server) GenerateExternalAuditStorageOIDCToken(ctx context.Context, integration string) (string, error) {
	token, err := awsoidc.GenerateAWSOIDCToken(ctx, a, a.GetKeyStore(), awsoidc.GenerateAWSOIDCTokenRequest{
		Integration: integration,
		Username:    a.ServerID,
		Subject:     types.IntegrationAWSOIDCSubjectAuth,
		Clock:       a.clock,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ExternalAuditStorageAuthenticateEvent{})

	return token, nil
}
