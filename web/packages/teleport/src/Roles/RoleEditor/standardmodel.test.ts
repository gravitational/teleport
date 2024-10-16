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

import { Role } from 'teleport/services/resources';

import {
  RoleEditorModel,
  roleEditorModelToRole,
  roleToRoleEditorModel,
} from './standardmodel';
import { withDefaults } from './withDefaults';

const minimalRole = () =>
  withDefaults({ metadata: { name: 'foobar' }, version: 'v7' });

const minimalRoleModel = (): RoleEditorModel => ({
  metadata: { name: 'foobar' },
  accessSpecs: [],
  requiresReset: false,
});

describe('roleToRoleEditorModel', () => {
  it('converts a minimal role', () => {
    expect(roleToRoleEditorModel(minimalRole())).toEqual(minimalRoleModel());
  });

  it('detects unknown fields', () => {
    const minRole = minimalRole();
    const roleModelWithReset: RoleEditorModel = {
      ...minimalRoleModel(),
      requiresReset: true,
    };

    expect(roleToRoleEditorModel(minRole).requiresReset).toEqual(false);

    expect(
      roleToRoleEditorModel({ ...minRole, unknownField: 123 } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        metadata: { name: 'foobar', unknownField: 123 },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: { ...minRole.spec, unknownField: 123 },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: { ...minRole.spec.allow, unknownField: 123 },
        },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          deny: { ...minRole.spec.deny, unknownField: 123 },
        },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          deny: { ...minRole.spec.deny, unknownField: 123 },
        },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          options: { ...minRole.spec.options, unknownField: 123 },
        },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            idp: { saml: { enabled: true }, unknownField: 123 },
          },
        },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            idp: { saml: { enabled: true, unknownField: 123 } },
          },
        },
      } as Role)
    ).toEqual(roleModelWithReset);

    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            record_session: {
              ...minRole.spec.options.record_session,
              unknownField: 123,
            },
          },
        },
      } as Role)
    ).toEqual(roleModelWithReset);
  });

  test('version change requires reset', () => {
    expect(roleToRoleEditorModel({ ...minimalRole(), version: 'v1' })).toEqual({
      ...minimalRoleModel(),
      requiresReset: true,
    } as RoleEditorModel);
  });

  it('converts metadata', () => {
    expect(
      roleToRoleEditorModel({
        ...minimalRole(),
        metadata: {
          name: 'role-name',
          description: 'role-description',
        },
      })
    ).toEqual({
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        description: 'role-description',
      },
    } as RoleEditorModel);
  });

  it('preserves original revision', () => {
    const exampleRole = () => ({
      ...minimalRole(),
      metadata: {
        name: 'role-name',
        revision: '5d7e724b-a52c-4c12-9372-60a8d1af5d33',
      },
    });
    expect(
      roleToRoleEditorModel(
        exampleRole(),
        exampleRole() // original
      )
    ).toEqual({
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        revision: '5d7e724b-a52c-4c12-9372-60a8d1af5d33',
      },
    } as RoleEditorModel);
  });

  test('revision change requires reset', () => {
    expect(
      roleToRoleEditorModel(
        {
          ...minimalRole(),
          metadata: {
            name: 'role-name',
            revision: '5d7e724b-a52c-4c12-9372-60a8d1af5d33',
          },
        },
        {
          ...minimalRole(),
          metadata: {
            name: 'role-name',
            revision: 'e39ea9f1-79b7-4d28-8f0c-af6848f9e655',
          },
        }
      )
    ).toEqual({
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        revision: 'e39ea9f1-79b7-4d28-8f0c-af6848f9e655',
      },
      requiresReset: true,
    } as RoleEditorModel);
  });

  it('converts server access spec', () => {
    const minRole = minimalRole();
    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            node_labels: { foo: 'bar', doubleFoo: ['bar1', 'bar2'] },
            logins: ['root', 'cthulhu', 'sandman'],
          },
        },
      })
    ).toEqual({
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'node',
          labels: [
            { name: 'foo', value: 'bar' },
            { name: 'doubleFoo', value: 'bar1' },
            { name: 'doubleFoo', value: 'bar2' },
          ],
          logins: [
            { label: 'root', value: 'root' },
            { label: 'cthulhu', value: 'cthulhu' },
            { label: 'sandman', value: 'sandman' },
          ],
        },
      ],
    } as RoleEditorModel);
  });
});

describe('roleEditorModelToRole', () => {
  it('converts metadata', () => {
    expect(
      roleEditorModelToRole({
        ...minimalRoleModel(),
        metadata: {
          name: 'dog-walker',
          description: 'walks dogs',
          revision: 'e2a3ccf8-09b9-4d97-8e47-6dbe3d53c0e5',
        },
      })
    ).toEqual({
      ...minimalRole(),
      metadata: {
        name: 'dog-walker',
        description: 'walks dogs',
        revision: 'e2a3ccf8-09b9-4d97-8e47-6dbe3d53c0e5',
      },
    } as Role);
  });

  it('converts a server access spec', () => {
    const minRole = minimalRole();
    expect(
      roleEditorModelToRole({
        ...minimalRoleModel(),
        accessSpecs: [
          {
            kind: 'node',
            labels: [
              { name: 'foo', value: 'bar' },
              { name: 'tripleFoo', value: 'bar1' },
              { name: 'tripleFoo', value: 'bar2' },
              { name: 'tripleFoo', value: 'bar3' },
            ],
            logins: [
              { label: 'root', value: 'root' },
              { label: 'cthulhu', value: 'cthulhu' },
              { label: 'sandman', value: 'sandman' },
            ],
          },
        ],
      })
    ).toEqual({
      ...minRole,
      spec: {
        ...minRole.spec,
        allow: {
          node_labels: { foo: 'bar', tripleFoo: ['bar1', 'bar2', 'bar3'] },
          logins: ['root', 'cthulhu', 'sandman'],
        },
      },
    } as Role);
  });
});
