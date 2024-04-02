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

package services

import (
	"github.com/gravitational/teleport/api/client/secreport"
	"github.com/gravitational/teleport/api/types"
)

// Services collects all services
type Services interface {
	UsersService
	Provisioner
	Trust
	types.Events
	ClusterConfiguration
	Access
	DynamicAccessCore
	Presence
	Restrictions
	Apps
	Databases
	DatabaseServices
	Kubernetes
	AppSession
	SnowflakeSession
	SAMLIdPSession
	types.WebSessionsGetter
	types.WebTokensGetter
	WindowsDesktops
	SAMLIdPServiceProviders
	UserGroups
	Integrations
	KubeWaitingContainer
	Notifications

	OktaClient() Okta
	AccessListClient() AccessLists
	AccessMonitoringRuleClient() AccessMonitoringRules
	UserLoginStateClient() UserLoginStates
	DiscoveryConfigClient() DiscoveryConfigs
	SecReportsClient() *secreport.Client
}

// RotationGetter returns the rotation state.
type RotationGetter func(role types.SystemRole) (*types.Rotation, error)
