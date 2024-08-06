/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package services

import (
	"context"

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
	services.ClusterConfiguration
	services.Restrictions
	services.Apps
	services.Kubernetes
	services.Databases
	services.DatabaseServices
	services.WindowsDesktops
	services.SAMLIdPServiceProviders
	services.UserGroups
	services.SessionTrackerService
	services.ConnectionsDiagnostic
	services.StatusInternal
	services.Integrations
	services.IntegrationsTokenGenerator
	services.DiscoveryConfigs
	services.Okta
	services.AccessLists
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
}

// GetWebSession returns existing web session described by req.
// Implements ReadAccessPoint
func (r *Services) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return r.Identity.WebSessions().Get(ctx, req)
}

// GetWebToken returns existing web token described by req.
// Implements ReadAccessPoint
func (r *Services) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return r.Identity.WebTokens().Get(ctx, req)
}

// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
func (r *Services) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	return r.IntegrationsTokenGenerator.GenerateAWSOIDCToken(ctx, integration)
}
