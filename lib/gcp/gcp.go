/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
