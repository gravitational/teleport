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
  | 'AccessGraph'
  | 'AccessGraphDemoMode'
  | 'AccessLists'
  | 'AccessMonitoring'
  | 'AccessRequests'
  | 'ActivityCenter'
  | 'App'
  | 'Beams'
  | 'ClientIPRestrictions'
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
  | 'SessionSummaries'
  | 'UnrestrictedManagedUpdates'
  | 'UpsellAlert'
  | 'UsageReporting';

type EntitlementInfo = { enabled: boolean; limit: number };
type LegacyPolicyConfig = {
  isPolicyEnabled?: boolean;
  entitlements?: Partial<Record<entitlement, EntitlementInfo>>;
};

const legacyPolicyFallbackEntitlements = [
  'AccessGraph',
  'ActivityCenter',
  'SessionSummaries',
] as const satisfies readonly entitlement[];

export const defaultEntitlements: Record<entitlement, EntitlementInfo> = {
  AccessGraph: { enabled: false, limit: 0 },
  AccessGraphDemoMode: { enabled: false, limit: 0 },
  AccessLists: { enabled: false, limit: 0 },
  AccessMonitoring: { enabled: false, limit: 0 },
  AccessRequests: { enabled: false, limit: 0 },
  ActivityCenter: { enabled: false, limit: 0 },
  App: { enabled: false, limit: 0 },
  Beams: { enabled: false, limit: 0 },
  ClientIPRestrictions: { enabled: false, limit: 0 },
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
  SessionSummaries: { enabled: false, limit: 0 },
  UnrestrictedManagedUpdates: { enabled: false, limit: 0 },
  UpsellAlert: { enabled: false, limit: 0 },
  UsageReporting: { enabled: false, limit: 0 },
};

/**
 * Applies the Identity Security entitlement split for config payloads from
 * older proxies. The fallback is applied only if all split entitlements are
 * absent.
 */
export function applyLegacyPolicyEntitlementFallback<
  T extends LegacyPolicyConfig,
>(config: T): T {
  const incomingEntitlements = config.entitlements ?? {};
  const hasPolicyEntitlement = Object.prototype.hasOwnProperty.call(
    incomingEntitlements,
    'Policy'
  );
  const policyEnabled = hasPolicyEntitlement
    ? incomingEntitlements?.Policy?.enabled === true
    : config.isPolicyEnabled === true;
  const hasSplitEntitlement = legacyPolicyFallbackEntitlements.some(
    entitlement =>
      Object.prototype.hasOwnProperty.call(incomingEntitlements, entitlement)
  );

  if (!policyEnabled || hasSplitEntitlement) {
    return config;
  }

  const entitlements = { ...incomingEntitlements };
  if (!hasPolicyEntitlement) {
    entitlements.Policy = { enabled: true, limit: 0 };
  }

  for (const entitlement of legacyPolicyFallbackEntitlements) {
    entitlements[entitlement] = { enabled: true, limit: 0 };
  }

  return { ...config, entitlements };
}
