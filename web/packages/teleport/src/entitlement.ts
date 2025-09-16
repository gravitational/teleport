/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

// entitlement list should be 1:1 with EntitlementKinds in entitlements/entitlements.go
type entitlement =
  | 'AccessLists'
  | 'AccessMonitoring'
  | 'AccessRequests'
  | 'App'
  | 'CloudAuditLogRetention'
  | 'DB'
  | 'Desktop'
  | 'DeviceTrust'
  | 'ExternalAuditStorage'
  | 'FeatureHiding'
  | 'HSM'
  | 'Identity'
  | 'JoinActiveSessions'
  | 'K8s'
  | 'MobileDeviceManagement'
  | 'OIDC'
  | 'OktaSCIM'
  | 'OktaUserSync'
  | 'Policy'
  | 'SAML'
  | 'SessionLocks'
  | 'UpsellAlert'
  | 'UsageReporting';

export const defaultEntitlements: Record<
  entitlement,
  { enabled: boolean; limit: number }
> = {
  AccessLists: { enabled: false, limit: 0 },
  AccessMonitoring: { enabled: false, limit: 0 },
  AccessRequests: { enabled: false, limit: 0 },
  App: { enabled: false, limit: 0 },
  CloudAuditLogRetention: { enabled: false, limit: 0 },
  DB: { enabled: false, limit: 0 },
  Desktop: { enabled: false, limit: 0 },
  DeviceTrust: { enabled: false, limit: 0 },
  ExternalAuditStorage: { enabled: false, limit: 0 },
  FeatureHiding: { enabled: false, limit: 0 },
  HSM: { enabled: false, limit: 0 },
  Identity: { enabled: false, limit: 0 },
  JoinActiveSessions: { enabled: false, limit: 0 },
  K8s: { enabled: false, limit: 0 },
  MobileDeviceManagement: { enabled: false, limit: 0 },
  OIDC: { enabled: false, limit: 0 },
  OktaSCIM: { enabled: false, limit: 0 },
  OktaUserSync: { enabled: false, limit: 0 },
  Policy: { enabled: false, limit: 0 },
  SAML: { enabled: false, limit: 0 },
  SessionLocks: { enabled: false, limit: 0 },
  UpsellAlert: { enabled: false, limit: 0 },
  UsageReporting: { enabled: false, limit: 0 },
};
