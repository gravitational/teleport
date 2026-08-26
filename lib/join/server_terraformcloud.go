/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package join

import (
	"context"

	"github.com/gravitational/trace"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/join/terraformcloud"
)

// validateTerraformCloudToken performs validation and allow rule verification
// against a Terraform Cloud OIDC token.
func (a *Server) validateTerraformCloudToken(
	ctx context.Context,
	pt provision.Token,
	idToken []byte,
) (any, *workloadidentityv1.JoinAttrs, error) {
	clusterName, err := a.cfg.AuthService.GetClusterName(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	claims, err := terraformcloud.CheckIDToken(ctx, &terraformcloud.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        idToken,
		Validator:      a.cfg.AuthService.GetTerraformIDTokenValidator(),
		ClusterName:    clusterName.GetClusterName(),
	})

	// If possible, attach claims and workload ID attrs regardless of the error
	// return. If the token fails to validate, these claims will ensure audit
	// events remain useful.
	var workloadIDAttrs *workloadidentityv1.JoinAttrs
	if claims != nil {
		workloadIDAttrs = &workloadidentityv1.JoinAttrs{
			TerraformCloud: claims.JoinAttrs(),
		}
	}

	return claims, workloadIDAttrs, trace.Wrap(err)
}
