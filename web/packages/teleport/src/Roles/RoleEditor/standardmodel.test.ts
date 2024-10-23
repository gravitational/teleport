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

import { Label as UILabel } from 'teleport/components/LabelsInput/LabelsInput';

import { Labels } from 'teleport/services/resources';

import {
  labelsModelToLabels,
  labelsToModel,
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

// These tests make sure that role to model and model to role conversions are
// symmetrical in typical cases.
describe.each<{ name: string; role: Role; model: RoleEditorModel }>([
  { name: 'minimal role', role: minimalRole(), model: minimalRoleModel() },

  {
    name: 'metadata',
    role: {
      ...minimalRole(),
      metadata: {
        name: 'role-name',
        description: 'role-description',
      },
    },
    model: {
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        description: 'role-description',
      },
    },
  },

  {
    name: 'server access spec',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          node_labels: { foo: 'bar' },
          logins: ['root', 'cthulhu', 'sandman'],
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'node',
          labels: [{ name: 'foo', value: 'bar' }],
          logins: [
            { label: 'root', value: 'root' },
            { label: 'cthulhu', value: 'cthulhu' },
            { label: 'sandman', value: 'sandman' },
          ],
        },
      ],
    },
  },

  {
    name: 'app access spec',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          app_labels: { foo: 'bar' },
          aws_role_arns: [
            'arn:aws:iam::123456789012:role/role1',
            'arn:aws:iam::123456789012:role/role2',
          ],
          azure_identities: [
            '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1',
            '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id2',
          ],
          gcp_service_accounts: [
            'account1@some-project.iam.gserviceaccount.com',
            'account2@some-project.iam.gserviceaccount.com',
          ],
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'app',
          labels: [{ name: 'foo', value: 'bar' }],
          awsRoleARNs: [
            'arn:aws:iam::123456789012:role/role1',
            'arn:aws:iam::123456789012:role/role2',
          ],
          azureIdentities: [
            '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1',
            '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id2',
          ],
          gcpServiceAccounts: [
            'account1@some-project.iam.gserviceaccount.com',
            'account2@some-project.iam.gserviceaccount.com',
          ],
        },
      ],
    },
  },

  {
    name: 'database access spec',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          db_labels: { env: 'prod' },
          db_names: ['stuff', 'knickknacks'],
          db_users: ['joe', 'mary'],
          db_roles: ['admin', 'auditor'],
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'db',
          labels: [{ name: 'env', value: 'prod' }],
          names: [
            { label: 'stuff', value: 'stuff' },
            { label: 'knickknacks', value: 'knickknacks' },
          ],
          users: [
            { label: 'joe', value: 'joe' },
            { label: 'mary', value: 'mary' },
          ],
          roles: [
            { label: 'admin', value: 'admin' },
            { label: 'auditor', value: 'auditor' },
          ],
        },
      ],
    },
  },

  {
    name: 'Windows desktop access spec',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          windows_desktop_labels: { os: 'WindowsForWorkgroups' },
          windows_desktop_logins: ['alice', 'bob'],
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'windows_desktop',
          labels: [{ name: 'os', value: 'WindowsForWorkgroups' }],
          logins: [
            { label: 'alice', value: 'alice' },
            { label: 'bob', value: 'bob' },
          ],
        },
      ],
    },
  },
])('$name', ({ role, model }) => {
  it('is converted to a model', () => {
    expect(roleToRoleEditorModel(role)).toEqual(model);
  });

  it('is created from a model', () => {
    expect(roleEditorModelToRole(model)).toEqual(role);
  });
});

describe('roleToRoleEditorModel', () => {
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

  it('preserves original revision', () => {
    const rev = '5d7e724b-a52c-4c12-9372-60a8d1af5d33';
    const originalRev = '9c2d5732-c514-46c3-b18d-2009b65af7b8';
    const exampleRole = (revision: string) => ({
      ...minimalRole(),
      metadata: {
        name: 'role-name',
        revision,
      },
    });
    expect(
      roleToRoleEditorModel(
        exampleRole(rev),
        exampleRole(originalRev) // original
      )
    ).toEqual({
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        revision: originalRev,
      },
      requiresReset: true,
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

  // This case has to be tested separately because of dynamic resource ID
  // generation.
  it('creates a Kubernetes access spec', () => {
    const minRole = minimalRole();
    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            kubernetes_groups: ['group1', 'group2'],
            kubernetes_labels: { bar: 'foo' },
            kubernetes_resources: [
              {
                kind: 'pod',
                name: 'some-pod',
                namespace: '*',
                verbs: ['get', 'update'],
              },
              {
                // No namespace required for cluster-wide resources.
                kind: 'kube_node',
                name: 'some-node',
              },
            ],
          },
        },
      })
    ).toEqual({
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'kube_cluster',
          groups: [
            { label: 'group1', value: 'group1' },
            { label: 'group2', value: 'group2' },
          ],
          labels: [{ name: 'bar', value: 'foo' }],
          resources: [
            {
              id: expect.any(String),
              kind: { value: 'pod', label: 'Pod' },
              name: 'some-pod',
              namespace: '*',
              verbs: [
                { label: 'get', value: 'get' },
                { label: 'update', value: 'update' },
              ],
            },
            {
              id: expect.any(String),
              kind: { value: 'kube_node', label: 'Node' },
              name: 'some-node',
              namespace: '',
              verbs: [],
            },
          ],
        },
      ],
    } as RoleEditorModel);
  });

  // Make sure that some fields are optional.
  it('creates a minimal app access spec', () => {
    const minRole = minimalRole();
    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            app_labels: { foo: 'bar' },
          },
        },
      })
    ).toEqual({
      ...minimalRoleModel(),
      accessSpecs: [
        {
          kind: 'app',
          labels: [{ name: 'foo', value: 'bar' }],
          awsRoleARNs: [],
          azureIdentities: [],
          gcpServiceAccounts: [],
        },
      ],
    } as RoleEditorModel);
  });
});

test('labelsToModel', () => {
  expect(labelsToModel({ foo: 'bar', doubleFoo: ['bar1', 'bar2'] })).toEqual([
    { name: 'foo', value: 'bar' },
    { name: 'doubleFoo', value: 'bar1' },
    { name: 'doubleFoo', value: 'bar2' },
  ]);
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

  // This case has to be tested separately because of dynamic resource ID
  // generation.
  it('converts a Kubernetes access spec', () => {
    const minRole = minimalRole();
    expect(
      roleEditorModelToRole({
        ...minimalRoleModel(),
        accessSpecs: [
          {
            kind: 'kube_cluster',
            groups: [
              { label: 'group1', value: 'group1' },
              { label: 'group2', value: 'group2' },
            ],
            labels: [{ name: 'bar', value: 'foo' }],
            resources: [
              {
                id: 'dummy-id-1',
                kind: { value: 'pod', label: 'Pod' },
                name: 'some-pod',
                namespace: '*',
                verbs: [
                  { label: 'get', value: 'get' },
                  { label: 'update', value: 'update' },
                ],
              },
              {
                id: 'dummy-id-2',
                kind: { value: 'kube_node', label: 'Node' },
                name: 'some-node',
                namespace: '',
                verbs: [],
              },
            ],
          },
        ],
      })
    ).toEqual({
      ...minRole,
      spec: {
        ...minRole.spec,
        allow: {
          kubernetes_groups: ['group1', 'group2'],
          kubernetes_labels: { bar: 'foo' },
          kubernetes_resources: [
            {
              kind: 'pod',
              name: 'some-pod',
              namespace: '*',
              verbs: ['get', 'update'],
            },
            {
              kind: 'kube_node',
              name: 'some-node',
              namespace: '',
              verbs: [],
            },
          ],
        },
      },
    } as Role);
  });
});

test('labelsModelToLabels', () => {
  const model: UILabel[] = [
    { name: 'foo', value: 'bar' },
    { name: 'doubleFoo', value: 'bar1' },
    { name: 'doubleFoo', value: 'bar2' },
    // Moving from 2 to 3 values is a separate code branch, hence one more
    // case.
    { name: 'tripleFoo', value: 'bar1' },
    { name: 'tripleFoo', value: 'bar2' },
    { name: 'tripleFoo', value: 'bar3' },
  ];
  expect(labelsModelToLabels(model)).toEqual({
    foo: 'bar',
    doubleFoo: ['bar1', 'bar2'],
    tripleFoo: ['bar1', 'bar2', 'bar3'],
  } as Labels);
});
