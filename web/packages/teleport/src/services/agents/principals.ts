/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

/** Principal dimension keys, matching inline resource constraint keys */
export type PrincipalType =
  | 'logins'
  | 'role_arns'
  | 'db_users'
  | 'db_names'
  | 'db_roles'
  | 'kube_groups'
  | 'kube_users'
  | 'azure_identities'
  | 'gcp_service_accounts';

/** Subset of one principal dimension's values granted by a single role */
export type RolePrincipalValues = {
  role: string;
  requiresRequest?: boolean;
  values?: string[];
};

/**
 * ResourcePrincipalSet is one principal dimension of a resource from the
 * unified resources listing, split into granted and requestable values.
 * byRole is only present for kinds whose dimensions must be co-granted
 * by a single role.
 */
export type ResourcePrincipalSet = {
  principalType: PrincipalType;
  granted?: string[];
  requestable?: string[];
  byRole?: RolePrincipalValues[];
};

/** A single principal value with per-value requestability */
export type PrincipalValue = {
  name: string;
  requiresRequest?: boolean;
};

/** Returns the principal set of the given dimension, if present */
export function principalSetOfType(
  resource: { principals?: ResourcePrincipalSet[] },
  type: PrincipalType
): ResourcePrincipalSet | undefined {
  return resource.principals?.find(p => p.principalType === type);
}

/** Returns the given dimension's values with per-value requestability */
export function principalsOfType(
  resource: { principals?: ResourcePrincipalSet[] },
  type: PrincipalType
): PrincipalValue[] {
  const set = principalSetOfType(resource, type);
  if (!set) {
    return [];
  }
  return [
    ...(set.granted ?? []).map(name => ({ name })),
    ...(set.requestable ?? []).map(name => ({ name, requiresRequest: true })),
  ];
}
