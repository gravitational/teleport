/*
Copyright 2022 Gravitational, Inc.

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

import "fmt"

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
