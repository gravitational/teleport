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

import type { DatabaseRolePrincipalGroup } from './types';

/** The reserved wildcard principal value. */
export const WildcardPrincipal = '*';

export type DatabasePrincipalDimension = 'users' | 'names' | 'roles';

/** A draft database constraint selection, flattened per dimension. */
export type DatabasePrincipalSelection = {
  users: string[];
  names: string[];
  roles: string[];
};

/**
 * Returns whether a single role grants the given value for a dimension.
 * A wildcard grant covers any literal for users/names, but a requested
 * wildcard is only covered by a wildcard grant. db_roles have no wildcard
 * matching.
 */
const groupGrants = (
  group: DatabaseRolePrincipalGroup,
  dimension: DatabasePrincipalDimension,
  value: string
): boolean => {
  const values = group[dimension] ?? [];
  if (value === WildcardPrincipal || dimension === 'roles') {
    return values.includes(value);
  }
  return values.includes(value) || values.includes(WildcardPrincipal);
};

/**
 * Returns whether adding the given value to the selection keeps it
 * satisfiable, mirroring the backend's database constraint validation
 * (ValidateDatabaseConstraintCoverage): a wildcard must be a dimension's
 * only value, and when both users and names are requested, every value
 * must pair with a value from the other dimension under a single granting
 * role, since session-time matchers are evaluated per role.
 *
 * Selected values themselves are not gated; deselection is always allowed.
 */
export const isDatabasePrincipalSelectable = (
  byRole: Record<string, DatabaseRolePrincipalGroup>,
  selection: DatabasePrincipalSelection,
  dimension: DatabasePrincipalDimension,
  value: string
): boolean => {
  const groups = Object.values(byRole);
  const selected = selection[dimension];

  // A wildcard subsumes the dimension's literals: it must be the only
  // value, in either selection order.
  if (dimension !== 'roles') {
    if (
      value === WildcardPrincipal &&
      selected.some(v => v !== WildcardPrincipal)
    ) {
      return false;
    }
    if (value !== WildcardPrincipal && selected.includes(WildcardPrincipal)) {
      return false;
    }
  }

  // The value must be granted by some role at all (a requested wildcard
  // only by a wildcard grant).
  if (!groups.some(g => groupGrants(g, dimension, value))) {
    return false;
  }

  // Combination coverage applies between users and names only, and only
  // once both dimensions would be non-empty: the value must pair with at
  // least one selected value from the other dimension under one role.
  const paired: DatabasePrincipalDimension | undefined =
    dimension === 'users' ? 'names' : dimension === 'names' ? 'users' : undefined;
  if (!paired || selection[paired].length === 0) {
    return true;
  }
  return selection[paired].some(other =>
    groups.some(
      g => groupGrants(g, dimension, value) && groupGrants(g, paired, other)
    )
  );
};
