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

package spacelift

import (
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// IDTokenClaims
// See the following for the structure:
// https://docs.spacelift.io/integrations/cloud-providers/oidc/#standard-claims
type IDTokenClaims struct {
	// Sub provides some information about the Spacelift run that generated this
	// token.
	// space:<space_id>:(stack|module):<stack_id|module_id>:run_type:<run_type>:scope:<read|write>
	Sub string `json:"sub"`
	// SpaceID is the ID of the space in which the run that owns the token was
	// executed.
	SpaceID string `json:"spaceId"`
	// CallerType is the type of the caller, ie. the entity that owns the run -
	// either stack or module.
	CallerType string `json:"callerType"`
	// CallerID is the ID of the caller, ie. the stack or module that generated
	// the run.
	CallerID string `json:"callerId"`
	// RunType is the type of the run.
	// (PROPOSED, TRACKED, TASK, TESTING or DESTROY)
	RunType string `json:"runType"`
	// RunID is the ID of the run that owns the token.
	RunID string `json:"runId"`
	// Scope is the scope of the token - either read or write.
	Scope string `json:"scope"`
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsSpacelift {
	return &workloadidentityv1pb.JoinAttrsSpacelift{
		Sub:        c.Sub,
		SpaceId:    c.SpaceID,
		CallerType: c.CallerType,
		CallerId:   c.CallerID,
		RunType:    c.RunType,
		RunId:      c.RunID,
		Scope:      c.Scope,
	}
}
