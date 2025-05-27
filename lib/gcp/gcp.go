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

package gcp

import (
	"github.com/zitadel/oidc/v3/pkg/oidc"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// defaultIssuerHost is the issuer for GCP ID tokens.
const defaultIssuerHost = "accounts.google.com"

// ComputeEngine contains VM-specific token claims.
type ComputeEngine struct {
	// The ID of the instance's project.
	ProjectID string `json:"project_id"`
	// The instance's zone.
	Zone string `json:"zone"`
	// The instance's ID.
	InstanceID string `json:"instance_id"`
	// The instance's name.
	InstanceName string `json:"instance_name"`
}

// Google contains Google-specific token claims.
type Google struct {
	ComputeEngine ComputeEngine `json:"compute_engine"`
}

// IDTokenClaims is the set of claims in a GCP ID token. GCP documentation for
// claims can be found at
// https://cloud.google.com/compute/docs/instances/verifying-instance-identity#payload
type IDTokenClaims struct {
	oidc.TokenClaims
	// The email of the service account that this token was issued for.
	Email  string `json:"email"`
	Google Google `json:"google"`
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsGCP {
	attrs := &workloadidentityv1pb.JoinAttrsGCP{
		ServiceAccount: c.Email,
	}
	if c.Google.ComputeEngine.InstanceName != "" {
		attrs.Gce = &workloadidentityv1pb.JoinAttrsGCPGCE{
			Project: c.Google.ComputeEngine.ProjectID,
			Zone:    c.Google.ComputeEngine.Zone,
			Id:      c.Google.ComputeEngine.InstanceID,
			Name:    c.Google.ComputeEngine.InstanceName,
		}
	}

	return attrs
}
