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

package common

const (
	// TeleportNamespace is used as the namespace prefix for labels defined by Teleport which can
	// carry metadata such as cloud AWS account or instance. Those labels can be used for RBAC.
	//
	// If a label with this prefix is used in a config file, the associated feature must take into
	// account that the label might be removed, modified or could have been set by the user.
	//
	// See also types.TeleportInternalLabelPrefix and types.TeleportHiddenLabelPrefix.
	TeleportNamespace = "teleport.dev"

	// OriginLabel is a resource metadata label name used to identify a source
	// that the resource originates from.
	OriginLabel = TeleportNamespace + "/origin"

	// OriginDefaults is an origin value indicating that the resource was
	// constructed as a default value.
	OriginDefaults = "defaults"

	// OriginConfigFile is an origin value indicating that the resource is
	// derived from static configuration.
	OriginConfigFile = "config-file"

	// OriginDynamic is an origin value indicating that the resource was
	// committed as dynamic configuration.
	OriginDynamic = "dynamic"

	// OriginCloud is an origin value indicating that the resource was
	// imported from a cloud provider.
	OriginCloud = "cloud"

	// OriginKubernetes is an origin value indicating that the resource was
	// created from the Kubernetes Operator.
	OriginKubernetes = "kubernetes"

	// OriginOkta is an origin value indicating that the resource was
	// created from the Okta service.
	OriginOkta = "okta"

	// OriginSCIM is an Origin value indicating that a resource was provisioned
	// via a SCIM service
	OriginSCIM = "scim"

	// OriginIntegrationAWSOIDC is an origin value indicating that the resource was
	// created from the AWS OIDC Integration.
	OriginIntegrationAWSOIDC = "integration_awsoidc"

	// OriginDiscoveryKubernetes indicates that the resource was imported
	// from kubernetes cluster by discovery service.
	OriginDiscoveryKubernetes = "discovery-kubernetes"

	// OriginEntraID indicates that the resource was imported
	// from the Entra ID directory.
	OriginEntraID = "entra-id"

	// OriginAWSIdentityCenter indicates that the resource was
	// imported from the AWS Identity Center or created from
	// the AWS Identity Center plugin.
	OriginAWSIdentityCenter = "aws-identity-center"
)

// OriginValues lists all possible origin values.
var OriginValues = []string{
	OriginDefaults,
	OriginConfigFile,
	OriginDynamic,
	OriginCloud,
	OriginKubernetes,
	OriginOkta,
	OriginSCIM,
	OriginDiscoveryKubernetes,
	OriginEntraID,
	OriginAWSIdentityCenter,
}
