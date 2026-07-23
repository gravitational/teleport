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

import {
  isDatabasePrincipalSelectable,
  type DatabasePrincipalSelection,
} from './principals';

const byRole = {
  roleA: { users: ['alice'], names: ['sales'] },
  roleB: { users: ['bob'], names: ['billing'] },
  roleWild: { users: ['*'], names: ['reporting'], requiresRequest: true },
  roleDbRoles: { roles: ['reader'], names: ['sales'] },
};

const sel = (
  s?: Partial<DatabasePrincipalSelection>
): DatabasePrincipalSelection => ({
  users: [],
  names: [],
  roles: [],
  ...s,
});

describe('isDatabasePrincipalSelectable', () => {
  it('allows any granted value with no cross-dimension selection', () => {
    expect(isDatabasePrincipalSelectable(byRole, sel(), 'users', 'alice')).toBe(
      true
    );
    expect(isDatabasePrincipalSelectable(byRole, sel(), 'names', 'sales')).toBe(
      true
    );
    expect(
      isDatabasePrincipalSelectable(byRole, sel(), 'roles', 'reader')
    ).toBe(true);
  });

  it('rejects values granted by no role', () => {
    // Every literal user is covered by roleWild's wildcard grant, so use
    // dimensions without one.
    expect(
      isDatabasePrincipalSelectable(byRole, sel(), 'names', 'nonexistent')
    ).toBe(false);
    expect(isDatabasePrincipalSelectable(byRole, sel(), 'roles', 'writer')).toBe(
      false
    );
  });

  it('enforces single-role (user, name) pairing', () => {
    // alice pairs with sales via roleA.
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ names: ['sales'] }),
        'users',
        'alice'
      )
    ).toBe(true);
    // bob is granted, but no single role grants (bob, sales).
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ names: ['sales'] }),
        'users',
        'bob'
      )
    ).toBe(false);
    // A value pairs when any one selected value from the other dimension
    // is co-granted.
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ names: ['sales', 'billing'] }),
        'users',
        'bob'
      )
    ).toBe(true);
    // Symmetric direction: names gated against selected users.
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ users: ['alice'] }),
        'names',
        'billing'
      )
    ).toBe(false);
  });

  it('treats a wildcard grant as covering literals for pairing', () => {
    // roleWild grants (any user, reporting).
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ names: ['reporting'] }),
        'users',
        'dev1'
      )
    ).toBe(true);
  });

  it('covers a requested wildcard only via a wildcard grant', () => {
    expect(isDatabasePrincipalSelectable(byRole, sel(), 'users', '*')).toBe(
      true
    );
    expect(isDatabasePrincipalSelectable(byRole, sel(), 'names', '*')).toBe(
      false
    );
    // The wildcard-granting role participates in pairing like any other:
    // (*, sales) is not co-granted by one role.
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ names: ['sales'] }),
        'users',
        '*'
      )
    ).toBe(false);
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ names: ['reporting'] }),
        'users',
        '*'
      )
    ).toBe(true);
  });

  it('makes a wildcard mutually exclusive with the dimension literals', () => {
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ users: ['alice'] }),
        'users',
        '*'
      )
    ).toBe(false);
    expect(
      isDatabasePrincipalSelectable(
        byRole,
        sel({ users: ['*'], names: ['reporting'] }),
        'users',
        'dev1'
      )
    ).toBe(false);
  });
});
