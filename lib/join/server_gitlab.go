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
	"github.com/gravitational/teleport/lib/join/gitlab"
	"github.com/gravitational/teleport/lib/join/provision"
)

// validateGitlabToken performs validation and allow rule verification against
// a Gitlab OIDC token.
func (a *Server) validateGitlabToken(
	ctx context.Context,
	pt provision.Token,
	idToken []byte,
) (any, *workloadidentityv1.JoinAttrs, error) {
	claims, err := gitlab.CheckIDToken(ctx, &gitlab.CheckIDTokenParams{
		ProvisionToken: pt,
		IDToken:        idToken,
		Validator:      a.cfg.AuthService.GetGitlabIDTokenValidator(),
	})

	// If possible, attach claims and workload ID attrs regardless of the error
	// return. If the token fails to validate, these claims will ensure audit
	// events remain useful.
	var workloadIDAttrs *workloadidentityv1.JoinAttrs
	if claims != nil {
		workloadIDAttrs = &workloadidentityv1.JoinAttrs{
			Gitlab: claims.JoinAttrs(),
		}
	}

	return claims, workloadIDAttrs, trace.Wrap(err)
}
