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
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
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
	// The email of the service account that this token was issued for.
	Email  string `json:"email"`
	Google Google `json:"google"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *IDTokenClaims) JoinAuditAttributes() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}
