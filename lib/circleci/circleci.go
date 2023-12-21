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

// CircleCI workload identity
//
// Key values:
// Issuer of tokens: `https://oidc.circleci.com/org/ORGANIZATION_ID`
// Audience: `ORGANIZATION_ID`
//
// `iat` and `exp` will be included and should be respected.
//
// Useful references:
// - https://circleci.com/docs/openid-connect-tokens/

package circleci

import (
	"fmt"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
)

const IssuerURLTemplate = "https://oidc.circleci.com/org/%s"

func issuerURL(template, organizationID string) string {
	return fmt.Sprintf(template, organizationID)
}

// IDTokenClaims is the structure of claims contained with a CircleCI issued
// ID token.
// See https://circleci.com/docs/openid-connect-tokens/
type IDTokenClaims struct {
	// Sub identifies who is running the CircleCI job and where.
	// In the format of: `org/ORGANIZATION_ID/project/PROJECT_ID/user/USER_ID`
	Sub string `json:"sub"`
	// ContextIDs is a list of UUIDs for the contexts used in the job.
	ContextIDs []string `json:"oidc.circleci.com/context-ids"`
	// ProjectID is the ID of the project in which the job is running.
	ProjectID string `json:"oidc.circleci.com/project-id"`
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
