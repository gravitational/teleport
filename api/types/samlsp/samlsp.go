/*
Copyright 2024 Gravitational, Inc.

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

package samlsp

const (
	// GCPWorkforce is a SAML service provider preset name for Google Cloud Platform
	// Workforce Identity Federation.
	GCPWorkforce = "gcp-workforce"
	// Unspecified preset type is used in the Web UI to denote a generic SAML service
	// provider preset.
	Unspecified = "unspecified"
	// AWSIdentityCenter is a SAML service provider preset name for AWS
	// Identity Center.
	AWSIdentityCenter = "aws-identity-center"
)

const (
	// DefaultRelayStateGCPWorkforce is a default relay state for GCPWorkforce preset.
	DefaultRelayStateGCPWorkforce = "https://console.cloud.google/"
)
