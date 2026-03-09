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

package auth

import (
	"context"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// Services is a collection of services that are used by the auth server.
// Avoid using this type as a dependency and instead depend on the actual
// methods/services you need. It should really only be necessary to directly
// reference this type on auth.Server itself and on code that manages
// the lifecycle of the auth server.
type Services struct {
	services.TrustInternal
	services.PresenceInternal
	services.Provisioner
	services.Identity
	services.Access
	services.DynamicAccessExt
	services.ClusterConfigurationInternal
	services.Restrictions
	services.Applications
	services.Kubernetes
	services.Databases
	services.DatabaseServices
	services.WindowsDesktops
	services.DynamicWindowsDesktops
	services.SAMLIdPServiceProviders
	services.UserGroups
	services.SessionTrackerService
	services.ConnectionsDiagnostic
	services.Status
	services.Integrations
	services.IntegrationsTokenGenerator
	services.UserTasks
	services.DiscoveryConfigs
	services.Okta
	services.AccessListsInternal
	services.DatabaseObjectImportRules
	services.DatabaseObjects
	services.UserLoginStates
	services.UserPreferences
	services.PluginData
	services.SCIM
	services.Notifications
	usagereporter.UsageReporter
	types.Events
	events.AuditLogSessionStreamer
	services.SecReports
	services.KubeWaitingContainer
	services.AccessMonitoringRules
	services.CrownJewels
	services.BotInstance
	services.AccessGraphSecretsGetter
	services.DevicesGetter
	services.SPIFFEFederations
	services.StaticHostUser
	services.AutoUpdateService
	services.ProvisioningStates
	services.IdentityCenter
	services.Plugins
	services.PluginStaticCredentials
	services.GitServers
	services.WorkloadIdentities
	services.StableUNIXUsersInternal
	services.WorkloadIdentityX509Revocations
	services.WorkloadIdentityX509Overrides
	services.SigstorePolicies
	services.HealthCheckConfig
	services.AppAuthConfig
	services.BackendInfoService
	services.VnetConfigService
	RecordingEncryptionManager
	events.MultipartHandler
	services.Summarizer
	services.ScopedTokenService
	MFAService
	services.WorkloadClusterService
}

// MFAService defines the interface for managing MFA resources in the backend.
type MFAService interface {
	// CreateValidatedMFAChallenge stores a ValidatedMFAChallenge resource for a given target cluster.
	CreateValidatedMFAChallenge(
		ctx context.Context,
		targetCluster string,
		challenge *mfav1.ValidatedMFAChallenge,
	) (*mfav1.ValidatedMFAChallenge, error)

	// GetValidatedMFAChallenge retrieves a ValidatedMFAChallenge resource by target cluster and challenge name.
	GetValidatedMFAChallenge(
		ctx context.Context,
		targetCluster string,
		challengeName string,
	) (*mfav1.ValidatedMFAChallenge, error)

	// ListValidatedMFAChallenges lists ValidatedMFAChallenge resources for all users.
	ListValidatedMFAChallenges(
		ctx context.Context,
		pageSize int32,
		pageToken string,
		targetCluster string,
	) ([]*mfav1.ValidatedMFAChallenge, string, error)
}
