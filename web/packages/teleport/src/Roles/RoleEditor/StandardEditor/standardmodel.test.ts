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

import {
  CreateDBUserMode,
  CreateHostUserMode,
  KubernetesResource,
  RequireMFAType,
  ResourceKind,
  Role,
  Rule,
  SessionRecordingMode,
} from 'teleport/services/resources';

import { Label as UILabel } from 'teleport/components/LabelsInput/LabelsInput';

import { Labels } from 'teleport/services/resources';

import {
  KubernetesAccess,
  labelsModelToLabels,
  labelsToModel,
  RoleEditorModel,
  roleEditorModelToRole,
  roleToRoleEditorModel,
} from './standardmodel';
import { optionsWithDefaults, withDefaults } from './withDefaults';

const minimalRole = () =>
  withDefaults({ metadata: { name: 'foobar' }, version: 'v7' });

const minimalRoleModel = (): RoleEditorModel => ({
  metadata: { name: 'foobar', labels: [] },
  resources: [],
  rules: [],
  requiresReset: false,
  options: {
    maxSessionTTL: '30h0m0s',
    clientIdleTimeout: '',
    disconnectExpiredCert: false,
    requireMFAType: {
      value: false,
      label: 'No',
    },
    createHostUserMode: {
      value: '',
      label: 'Unspecified',
    },
    createDBUser: false,
    createDBUserMode: {
      value: '',
      label: 'Unspecified',
    },
    desktopClipboard: true,
    createDesktopUser: false,
    desktopDirectorySharing: true,
    defaultSessionRecordingMode: { value: 'best_effort', label: 'Best Effort' },
    sshSessionRecordingMode: { value: '', label: 'Unspecified' },
    recordDesktopSessions: true,
    forwardAgent: false,
  },
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
        labels: { foo: 'bar' },
      },
    },
    model: {
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        description: 'role-description',
        labels: [{ name: 'foo', value: 'bar' }],
      },
    },
  },

  {
    name: 'server access',
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
      resources: [
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
    name: 'app access',
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
      resources: [
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
    name: 'database access',
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
      resources: [
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
    name: 'Windows desktop access',
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
      resources: [
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

  {
    name: 'Options object',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        options: {
          ...minimalRole().spec.options,
          max_session_ttl: '1h15m30s',
          client_idle_timeout: '2h30m45s',
          disconnect_expired_cert: true,
          require_session_mfa: 'hardware_key',
          create_host_user_mode: 'keep',
          create_db_user: true,
          create_db_user_mode: 'best_effort_drop',
          desktop_clipboard: false,
          create_desktop_user: true,
          desktop_directory_sharing: false,
          record_session: {
            default: 'strict',
            desktop: false,
            ssh: 'best_effort',
          },
          forward_agent: true,
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      options: {
        maxSessionTTL: '1h15m30s',
        clientIdleTimeout: '2h30m45s',
        disconnectExpiredCert: true,
        requireMFAType: { value: 'hardware_key', label: 'Hardware Key' },
        createHostUserMode: { value: 'keep', label: 'Keep' },
        createDBUser: true,
        createDBUserMode: {
          value: 'best_effort_drop',
          label: 'Drop (best effort)',
        },
        desktopClipboard: false,
        createDesktopUser: true,
        desktopDirectorySharing: false,
        defaultSessionRecordingMode: { value: 'strict', label: 'Strict' },
        sshSessionRecordingMode: { value: 'best_effort', label: 'Best Effort' },
        recordDesktopSessions: false,
        forwardAgent: true,
      },
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
  const minRole = minimalRole();
  const roleModelWithReset: RoleEditorModel = {
    ...minimalRoleModel(),
    requiresReset: true,
  };
  // Same as newResourceAccess('kube_cluster'), but without default groups.
  const newKubeClusterResourceAccess = (): KubernetesAccess => ({
    kind: 'kube_cluster',
    groups: [],
    labels: [],
    resources: [],
  });

  test.each<{ name: string; role: Role; model?: RoleEditorModel }>([
    {
      name: 'unknown fields in Role',
      role: { ...minRole, unknownField: 123 } as Role,
    },

    {
      name: 'unknown fields in metadata',
      role: {
        ...minRole,
        metadata: { name: 'foobar', unknownField: 123 },
      } as Role,
    },

    {
      name: 'unknown fields in spec',
      role: {
        ...minRole,
        spec: { ...minRole.spec, unknownField: 123 },
      } as Role,
    },

    {
      name: 'unknown fields in spec.allow',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: { ...minRole.spec.allow, unknownField: 123 },
        },
      } as Role,
    },

    {
      name: 'unknown fields in KubernetesResource',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            kubernetes_resources: [
              { kind: 'job', unknownField: 123 } as KubernetesResource,
            ],
          },
        },
      } as Role,
      model: {
        ...roleModelWithReset,
        resources: [
          {
            ...newKubeClusterResourceAccess(),
            resources: [expect.any(Object)],
          },
        ],
      },
    },

    {
      name: 'unsupported resource kind in KubernetesResource',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            kubernetes_resources: [
              { kind: 'illegal' } as unknown as KubernetesResource,
              { kind: 'job' },
            ],
          },
        },
      } as Role,
      model: {
        ...roleModelWithReset,
        resources: [
          {
            ...newKubeClusterResourceAccess(),
            resources: [
              expect.objectContaining({ kind: { value: 'job', label: 'Job' } }),
            ],
          },
        ],
      },
    },

    {
      name: 'unsupported verb in KubernetesResource',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            kubernetes_resources: [
              {
                kind: '*',
                verbs: ['illegal', 'get'],
              } as unknown as KubernetesResource,
            ],
          },
        },
      } as Role,
      model: {
        ...roleModelWithReset,
        resources: [
          {
            ...newKubeClusterResourceAccess(),
            resources: [
              expect.objectContaining({
                verbs: [{ value: 'get', label: 'get' }],
              }),
            ],
          },
        ],
      },
    },

    {
      name: 'unknown fields in Rule',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            rules: [{ unknownField: 123 } as Rule],
          },
        },
      } as Role,
      model: {
        ...roleModelWithReset,
        rules: [expect.any(Object)],
      },
    },

    {
      name: 'unsupported verb in Rule',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            rules: [{ verbs: ['illegal', 'create'] } as unknown as Rule],
          },
        },
      } as Role,
      model: {
        ...roleModelWithReset,
        rules: [
          expect.objectContaining({
            verbs: [{ value: 'create', label: 'create' }],
          }),
        ],
      },
    },

    {
      name: 'unknown fields in spec.deny',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          deny: { ...minRole.spec.deny, unknownField: 123 },
        },
      } as Role,
    },

    {
      name: 'unknown fields in spec.options',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: { ...minRole.spec.options, unknownField: 123 },
        },
      } as Role,
    },

    {
      name: 'unknown fields in spec.options.idp.saml',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            idp: { saml: { enabled: true, unknownField: 123 } },
          },
        },
      } as Role,
    },

    {
      name: 'unknown fields in spec.options.record_session',
      role: {
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
      } as Role,
    },

    {
      name: 'unsupported value in spec.options.require_session_mfa',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            require_session_mfa: 'bogus' as RequireMFAType,
          },
        },
      },
      model: {
        ...roleModelWithReset,
        options: {
          ...roleModelWithReset.options,
          requireMFAType: { value: false, label: 'No' },
        },
      },
    },

    {
      name: 'unsupported value in spec.options.create_host_user_mode',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            create_host_user_mode: 'bogus' as CreateHostUserMode,
          },
        },
      },
      model: {
        ...roleModelWithReset,
        options: {
          ...roleModelWithReset.options,
          createHostUserMode: { value: '', label: 'Unspecified' },
        },
      },
    },

    {
      name: 'unsupported value in spec.options.create_db_user_mode',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: {
            ...minRole.spec.options,
            create_db_user_mode: 'bogus' as CreateDBUserMode,
          },
        },
      },
      model: {
        ...roleModelWithReset,
        options: {
          ...roleModelWithReset.options,
          createDBUserMode: { value: '', label: 'Unspecified' },
        },
      },
    },

    {
      name: 'unsupported value in spec.options.record_session.default',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: optionsWithDefaults({
            record_session: { default: 'bogus' as SessionRecordingMode },
          }),
        },
      },
      model: {
        ...roleModelWithReset,
        options: {
          ...roleModelWithReset.options,
          defaultSessionRecordingMode: { value: '', label: 'Unspecified' },
        },
      },
    },

    {
      name: 'unsupported value in spec.options.record_session.ssh',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: optionsWithDefaults({
            record_session: { ssh: 'bogus' as SessionRecordingMode },
          }),
        },
      },
      model: {
        ...roleModelWithReset,
        options: {
          ...roleModelWithReset.options,
          sshSessionRecordingMode: { value: '', label: 'Unspecified' },
        },
      },
    },
  ])(
    'requires reset because of $name',
    ({ role, model = roleModelWithReset }) => {
      expect(roleToRoleEditorModel(role)).toEqual(model);
    }
  );

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
        labels: [],
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
        labels: [],
      },
      requiresReset: true,
    } as RoleEditorModel);
  });

  // This case has to be tested separately because of dynamic resource ID
  // generation.
  it('creates Kubernetes access', () => {
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
      resources: [
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
  it('creates minimal app access', () => {
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
      resources: [
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

it('creates a rule model', () => {
  expect(
    roleToRoleEditorModel({
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          rules: [
            {
              resources: [ResourceKind.User, ResourceKind.DatabaseService],
              verbs: ['read', 'list'],
            },
            { resources: [ResourceKind.Lock], verbs: ['create'] },
          ],
        },
      },
    })
  ).toEqual({
    ...minimalRoleModel(),
    rules: [
      {
        id: expect.any(String),
        resources: [
          { label: 'user', value: 'user' },
          { label: 'db_service', value: 'db_service' },
        ],
        verbs: [
          { label: 'read', value: 'read' },
          { label: 'list', value: 'list' },
        ],
      },
      {
        id: expect.any(String),
        resources: [{ label: 'lock', value: 'lock' }],
        verbs: [{ label: 'create', value: 'create' }],
      },
    ],
  } as RoleEditorModel);
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
          labels: [{ name: 'kind', value: 'occupation' }],
        },
      })
    ).toEqual({
      ...minimalRole(),
      metadata: {
        name: 'dog-walker',
        description: 'walks dogs',
        revision: 'e2a3ccf8-09b9-4d97-8e47-6dbe3d53c0e5',
        labels: { kind: 'occupation' },
      },
    } as Role);
  });

  // This case has to be tested separately because of dynamic resource ID
  // generation.
  it('converts Kubernetes access', () => {
    const minRole = minimalRole();
    expect(
      roleEditorModelToRole({
        ...minimalRoleModel(),
        resources: [
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

  it('converts a rule model', () => {
    expect(
      roleEditorModelToRole({
        ...minimalRoleModel(),
        rules: [
          {
            id: 'dummy-id-1',
            resources: [
              { label: 'user', value: ResourceKind.User },
              { label: 'db_service', value: ResourceKind.DatabaseService },
            ],
            verbs: [
              { label: 'read', value: 'read' },
              { label: 'list', value: 'list' },
            ],
          },
          {
            id: 'dummy-id-2',
            resources: [{ label: 'lock', value: ResourceKind.Lock }],
            verbs: [{ label: 'create', value: 'create' }],
          },
        ],
      })
    ).toEqual({
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          rules: [
            { resources: ['user', 'db_service'], verbs: ['read', 'list'] },
            { resources: ['lock'], verbs: ['create'] },
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
