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

package gitlab

import (
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
)

// GitLab Workload Identity
//
// GL provides workloads with the ID token in an environment variable included
// in the workflow config, with the specified audience:
//
// ```yaml
// job-name:
//  id_tokens:
//    TBOT_GITLAB_JWT:
//      aud: https://teleport.example.com
// ```
//
// We will require the user to configure this to be `TBOT_GITLAB_JWT` and to
//
//
// Valuable reference:
// - https://docs.gitlab.com/ee/ci/yaml/index.html#id_tokens
// - https://docs.gitlab.com/ee/ci/cloud_services/
//
// The GitLab issuer's well-known OIDC document is at
// https://gitlab.com/.well-known/openid-configuration
// For GitLab self-hosted servers, this will be at
// https://$HOSTNAME/.well-known/openid-configuration

// IDTokenClaims is the structure of claims contained within a GitLab issued
// ID token.
//
// See the following for the structure:
// https://docs.gitlab.com/ee/ci/cloud_services/#how-it-works
type IDTokenClaims struct {
	// Sub also known as Subject is a string that roughly uniquely identifies
	// the workload. The format of this varies depending on the type of
	// github action run.
	Sub string `json:"sub"`
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
