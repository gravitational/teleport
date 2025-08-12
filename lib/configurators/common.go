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

package configurators

import (
	"context"
)

// TargetService is the target service for bootstrapping.
type TargetService int

const (
	// DatabaseService indicates the bootstrap is for database service. Cloud
	// matchers and static databases are scanned from `database_service` and
	// both discovery and access/auth permissions will be collected.
	DatabaseService TargetService = iota
	// DiscoveryService indicates the bootstrap is for discovery service. Cloud
	// matchers are scanned from `discovery_service` and discovery permissions
	// will be collected.
	DiscoveryService
	// DatabaseServiceByDiscoveryServiceConfig indicates the bootstrap is for
	// database service that is receiving dynamic/discovered resources from the
	// discovery service. Cloud matchers are scanned from `discovery_service`
	// and access/auth permissions will be collected.
	DatabaseServiceByDiscoveryServiceConfig
)

// Name returns the target service name.
func (t TargetService) Name() string {
	switch t {
	case DatabaseService,
		DatabaseServiceByDiscoveryServiceConfig:
		return "Database Service"
	case DiscoveryService:
		return "Discovery Service"
	default:
		return "unknown service"
	}
}

// IsDiscovery returns true if target is discovery service.
func (t TargetService) IsDiscovery() bool {
	return t == DiscoveryService
}

// UseDiscoveryServiceConfig returns true if target is using discovery service
// config.
func (t TargetService) UseDiscoveryServiceConfig() bool {
	return t == DiscoveryService || t == DatabaseServiceByDiscoveryServiceConfig
}

// BootstrapFlags flags provided by users to configure and define how the
// configurators will work.
type BootstrapFlags struct {
	// Service specifies the target service for bootstrapping.
	Service TargetService
	// ConfigPath database agent configuration path.
	ConfigPath string
	// Manual boolean indicating if the configurator will perform the
	// instructions or if it will be the user.
	Manual bool
	// PolicyName name of the generated policy.
	PolicyName string
	// AttachToUser user that the generated policies will be attached to.
	AttachToUser string
	// AttachToRole role that the generated policies will be attached to.
	AttachToRole string
	// ForceRDSPermissions forces the presence of RDS permissions.
	ForceRDSPermissions bool
	// ForceRDSProxyPermissions forces the presence of RDS Proxy permissions.
	ForceRDSProxyPermissions bool
	// ForceRedshiftPermissions forces the presence of Redshift permissions.
	ForceRedshiftPermissions bool
	// ForceRedshiftServerlessPermissions forces the presence of Redshift Serverless permissions.
	ForceRedshiftServerlessPermissions bool
	// ForceElastiCachePermissions forces the presence of ElastiCache permissions.
	ForceElastiCachePermissions bool
	// ForceMemoryDBPermissions forces the presence of MemoryDB permissions.
	ForceMemoryDBPermissions bool
	// ForceEC2Permissions forces the presence of EC2 permissions.
	ForceEC2Permissions bool
	// ForceAWSKeyspacesPermissions forces the presence of AWS Keyspaces permissions.
	ForceAWSKeyspacesPermissions bool
	// ForceDynamoDBPermissions forces the presence of DynamoDB permissions.
	ForceDynamoDBPermissions bool
	// ForceOpenSearchPermissions forces the presence of OpenSearch permissions.
	ForceOpenSearchPermissions bool
	// ForceDocumentDBPermissions forces the presence of DocumentDB permissions.
	ForceDocumentDBPermissions bool
	// Proxy is the address of the Teleport proxy to use.
	Proxy string
	// ForceAssumesRoles forces the presence of additional external AWS IAM roles to assume.
	ForceAssumesRoles string
	// AssumeRoleARN is the ARN of the role to assume while bootstrapping.
	AssumeRoleARN string
	// ExternalID is the external ID to use when assuming a role.
	ExternalID string
}

// ConfiguratorActionContext context passed across configurator actions. It is
// used to share attributes between actions.
type ConfiguratorActionContext struct {
	// AWSPolicyArn AWS ARN of the created policy.
	AWSPolicyArn string
}

// ConfiguratorAction is single configurator action, its details can be retrieved
// using `Description` and `Details`, and executed using `Execute` function.
type ConfiguratorAction interface {
	// Description returns human-readable description of what the action will
	// do.
	Description() string
	// Details if the action has some additional information, such as a JSON
	// payload, it will be returned in the `Details`.
	Details() string
	// Execute executes the action with the provided context. It might or not
	// modify the `ConfiguratorActionContext`.
	//
	// Actions can store and retrieve information from the
	// `ConfiguratorActionContext` that is passed to `Execute`. For example,
	// if an action requires information that was generated by a previous action.
	// It should retrieve this information from context.
	Execute(context.Context, *ConfiguratorActionContext) error
}

// Configurator responsible for generating a list of actions that needs to be
// performed in the database agent bootstrap process.
type Configurator interface {
	// Actions return the list of actions that needs to be performed by the
	// users (when in manual mode) or by the configurator itself.
	Actions() []ConfiguratorAction
	// Name returns the configurator name.
	Name() string
	// Description returns a brief description of the configurator.
	Description() string
	// IsEmpty defines if the configurator will have to perform any action.
	IsEmpty() bool
}
