// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package provision

import (
	"time"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
)

// A Token is used in the join service to facilitate provisioning.
type Token interface {
	// GetName returns the name of the token.
	GetName() string
	// GetSafeName returns the name of the token, sanitized appropriately for
	// join methods where the name is secret. This should be used when logging
	// the token name.
	GetSafeName() string
	// GetSecret returns either the token's secret or an empty string depending on whether
	// or not the token has an explicit secret value. It also returns an explicit boolean
	// to disambiguate between cases where the token has no secret or the token secret is
	// not populated.
	GetSecret() (string, bool)
	// GetJoinMethod returns joining method that must be used with this token.
	GetJoinMethod() types.JoinMethod
	// GetRoles returns a list of teleport roles that will be granted to the
	// resources provisioned with this token.
	GetRoles() types.SystemRoles
	// Expiry returns the token's expiration time.
	Expiry() time.Time
	// GetBotName returns the BotName field which must be set for joining bots.
	GetBotName() string
	// GetAssignedScope returns the scope that will be assigned to provisioned resources
	// provisioned using the wrapped [joiningv1.ScopedToken].
	GetAssignedScope() string
	// GetImmutableLabels returns labels that must be applied to resources
	// provisioned with this token.
	GetImmutableLabels() *joiningv1.ImmutableLabels
	// GetAWSAllowRules returns the list of AWS-specific allow rules.
	GetAWSAllowRules() []*types.TokenRule
	// GetAWSIIDTTL returns the TTL of EC2 IIDs
	GetAWSIIDTTL() types.Duration
	// GetIntegration returns the integration name that provides credentials to validate allow rules.
	// Currently, this is only used to validate the AWS Organization.
	GetIntegration() string
	// GetGCPRules returns the GCP-specific configuration for this token.
	GetGCPRules() *types.ProvisionTokenSpecV2GCP
	// GetAzure returns the Azure-specific configuration for this token.
	GetAzure() *types.ProvisionTokenSpecV2Azure
	// GetAzureDevops returns the AzureDevops-specific configuration for this token.
	GetAzureDevops() *types.ProvisionTokenSpecV2AzureDevops
	// GetOracle returns the Oracle-specific configuration for this token.
	GetOracle() *types.ProvisionTokenSpecV2Oracle
}
