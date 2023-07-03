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

package types

import "github.com/gravitational/trace"

const (
	// IntegrationAWSOIDCAudience is the client id used to generate the JWT.
	// This value must match the Audience defined in the IAM Identity Provider of the Integration.
	IntegrationAWSOIDCAudience = "discover.teleport"

	// IntegrationAWSOIDCSubject identifies the system that is going to use the token.
	IntegrationAWSOIDCSubject = "system:proxy"
)

// GenerateAWSOIDCTokenRequest are the parameters used to request an AWS OIDC Integration token.
type GenerateAWSOIDCTokenRequest struct {
	// Issuer is the entity that is signing the JWT.
	// This value must contain the AWS OIDC Integration configured provider (Proxy's Public URL).
	Issuer string `json:"issuer"`
}

// CheckAndSetDefaults checks if the required fields are present.
func (req *GenerateAWSOIDCTokenRequest) CheckAndSetDefaults() error {
	if req.Issuer == "" {
		return trace.BadParameter("issuer is required")
	}

	return nil
}
