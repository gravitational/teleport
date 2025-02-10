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

import { Label as UILabel } from 'teleport/components/LabelsInput/LabelsInput';
import {
  CreateDBUserMode,
  CreateHostUserMode,
  GitHubPermission,
  KubernetesResource,
  Labels,
  RequireMFAType,
  ResourceKind,
  Role,
  RoleVersion,
  Rule,
  SessionRecordingMode,
  SSHPortForwarding,
} from 'teleport/services/resources';

import presetRoles from '../../../../../../../gen/preset-roles.json';
import {
  createDBUserModeOptionsMap,
  createHostUserModeOptionsMap,
  defaultRoleVersion,
  gitHubOrganizationsToModel,
  KubernetesAccess,
  kubernetesResourceKindOptionsMap,
  kubernetesVerbOptionsMap,
  labelsModelToLabels,
  labelsToModel,
  portForwardingOptionsToModel,
  requireMFATypeOptionsMap,
  resourceKindOptionsMap,
  RoleEditorModel,
  roleEditorModelToRole,
  roleToRoleEditorModel,
  roleVersionOptionsMap,
  sessionRecordingModeOptionsMap,
  sshPortForwardingModeOptionsMap,
  verbOptionsMap,
} from './standardmodel';
import { optionsWithDefaults, withDefaults } from './withDefaults';

const minimalRole = () =>
  withDefaults({ metadata: { name: 'foobar' }, version: defaultRoleVersion });

const minimalRoleModel = (): RoleEditorModel => ({
  metadata: {
    name: 'foobar',
    labels: [],
    version: roleVersionOptionsMap.get(defaultRoleVersion),
  },
  resources: [],
  rules: [],
  requiresReset: false,
  options: {
    maxSessionTTL: '30h0m0s',
    clientIdleTimeout: '',
    disconnectExpiredCert: false,
    requireMFAType: requireMFATypeOptionsMap.get(false),
    createHostUserMode: createHostUserModeOptionsMap.get(''),
    createDBUser: false,
    createDBUserMode: createDBUserModeOptionsMap.get(''),
    desktopClipboard: true,
    createDesktopUser: false,
    desktopDirectorySharing: true,
    defaultSessionRecordingMode:
      sessionRecordingModeOptionsMap.get('best_effort'),
    sshSessionRecordingMode: sessionRecordingModeOptionsMap.get(''),
    recordDesktopSessions: true,
    forwardAgent: false,
    sshPortForwardingMode: sshPortForwardingModeOptionsMap.get(''),
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
      version: RoleVersion.V6,
    },
    model: {
      ...minimalRoleModel(),
      metadata: {
        name: 'role-name',
        description: 'role-description',
        labels: [{ name: 'foo', value: 'bar' }],
        version: roleVersionOptionsMap.get(RoleVersion.V6),
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
          db_service_labels: { foo: 'bar' },
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
          dbServiceLabels: [{ name: 'foo', value: 'bar' }],
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
    name: 'GitHub organizations',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        allow: {
          github_permissions: [{ orgs: ['illuminati', 'reptilians'] }],
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      resources: [
        {
          kind: 'git_server',
          organizations: [
            { label: 'illuminati', value: 'illuminati' },
            { label: 'reptilians', value: 'reptilians' },
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
          ssh_port_forwarding: {
            local: {
              enabled: true,
            },
            remote: {
              enabled: false,
            },
          },
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      options: {
        maxSessionTTL: '1h15m30s',
        clientIdleTimeout: '2h30m45s',
        disconnectExpiredCert: true,
        requireMFAType: requireMFATypeOptionsMap.get('hardware_key'),
        createHostUserMode: createHostUserModeOptionsMap.get('keep'),
        createDBUser: true,
        createDBUserMode: createDBUserModeOptionsMap.get('best_effort_drop'),
        desktopClipboard: false,
        createDesktopUser: true,
        desktopDirectorySharing: false,
        defaultSessionRecordingMode:
          sessionRecordingModeOptionsMap.get('strict'),
        sshSessionRecordingMode:
          sessionRecordingModeOptionsMap.get('best_effort'),
        recordDesktopSessions: false,
        forwardAgent: true,
        sshPortForwardingMode:
          sshPortForwardingModeOptionsMap.get('local-only'),
      },
    },
  },

  {
    name: 'Options object with legacy port forwarding',
    role: {
      ...minimalRole(),
      spec: {
        ...minimalRole().spec,
        options: {
          ...minimalRole().spec.options,
          port_forwarding: true,
        },
      },
    },
    model: {
      ...minimalRoleModel(),
      options: {
        ...minimalRoleModel().options,
        sshPortForwardingMode:
          sshPortForwardingModeOptionsMap.get('deprecated-on'),
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

test.each<{
  name: string;
  portForwarding?: SSHPortForwarding;
  legacyPortForwarding?: boolean;
  expected: ReturnType<typeof portForwardingOptionsToModel>;
}>([
  { name: 'unspecified', expected: { model: '', requiresReset: false } },
  {
    name: 'none',
    portForwarding: { local: { enabled: false }, remote: { enabled: false } },
    expected: { model: 'none', requiresReset: false },
  },
  {
    name: 'local-only',
    portForwarding: { local: { enabled: true }, remote: { enabled: false } },
    expected: { model: 'local-only', requiresReset: false },
  },
  {
    name: 'remote-only',
    portForwarding: { local: { enabled: false }, remote: { enabled: true } },
    expected: { model: 'remote-only', requiresReset: false },
  },
  {
    name: ' local-and-remote',
    portForwarding: { local: { enabled: true }, remote: { enabled: true } },
    expected: { model: 'local-and-remote', requiresReset: false },
  },
  {
    name: 'deprecated-on',
    legacyPortForwarding: true,
    expected: { model: 'deprecated-on', requiresReset: false },
  },
  {
    name: 'deprecated-off',
    legacyPortForwarding: false,
    expected: { model: 'deprecated-off', requiresReset: false },
  },
  {
    name: 'local-and-remote (overriding deprecated even if specified)',
    portForwarding: { local: { enabled: true }, remote: { enabled: true } },
    legacyPortForwarding: false,
    expected: { model: 'local-and-remote', requiresReset: false },
  },
  {
    name: 'an empty port forwarding object',
    portForwarding: {},
    expected: { model: '', requiresReset: true },
  },
  {
    name: 'empty local and remote objects',
    portForwarding: { local: {}, remote: {} },
    expected: { model: '', requiresReset: true },
  },
  {
    name: 'unknown fields',
    portForwarding: {
      local: { enabled: true },
      remote: { enabled: true },
      foo: 'bar',
    } as SSHPortForwarding,
    expected: { model: 'local-and-remote', requiresReset: true },
  },
  {
    name: 'unknown fields in local',
    portForwarding: {
      local: { enabled: true, foo: 'bar' },
      remote: { enabled: false },
    } as SSHPortForwarding,
    expected: { model: 'local-only', requiresReset: true },
  },
  {
    name: 'unknown fields in remote',
    portForwarding: {
      local: { enabled: false },
      remote: { enabled: false, foo: 'bar' },
    } as SSHPortForwarding,
    expected: { model: 'none', requiresReset: true },
  },
])(
  'portForwardingOptionsToModel(): $name',
  ({ portForwarding, legacyPortForwarding, expected }) => {
    expect(
      portForwardingOptionsToModel(portForwarding, legacyPortForwarding)
    ).toEqual(expected);
  }
);

test.each<{
  name: string;
  permissions: GitHubPermission[];
  expected: ReturnType<typeof gitHubOrganizationsToModel>;
}>([
  {
    name: 'empty permissions array',
    permissions: [],
    expected: { model: [], requiresReset: false },
  },
  {
    name: 'some organizations',
    permissions: [{ orgs: ['foo', 'bar'] }],
    expected: {
      model: [
        { label: 'foo', value: 'foo' },
        { label: 'bar', value: 'bar' },
      ],
      requiresReset: false,
    },
  },
  {
    name: 'empty permissions object',
    permissions: [{}],
    expected: {
      model: [],
      requiresReset: false,
    },
  },
  {
    name: 'empty organizations array',
    permissions: [{ orgs: [] }],
    expected: { model: [], requiresReset: false },
  },
  {
    name: 'multiple permission objects',
    permissions: [{ orgs: ['foo1', 'foo2'] }, { orgs: ['bar'] }],
    expected: {
      model: [
        { label: 'foo1', value: 'foo1' },
        { label: 'foo2', value: 'foo2' },
        { label: 'bar', value: 'bar' },
      ],
      requiresReset: false,
    },
  },
  {
    name: 'invalid fields',
    permissions: [{ orgs: ['foo'], unknownField: 123 } as GitHubPermission],
    expected: {
      model: [{ label: 'foo', value: 'foo' }],
      requiresReset: true,
    },
  },
])('gitHubOrganizationsToModel(): $name', ({ permissions, expected }) => {
  expect(gitHubOrganizationsToModel(permissions)).toEqual(expected);
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
    users: [],
    roleVersion: defaultRoleVersion,
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
                verbs: [kubernetesVerbOptionsMap.get('get')],
              }),
            ],
          },
        ],
      },
    },

    {
      name: 'unknown field in github_permissions',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            github_permissions: [
              { orgs: ['foo'], unknownField: 123 } as GitHubPermission,
            ],
          },
        },
      },
      model: {
        ...roleModelWithReset,
        resources: [
          {
            kind: 'git_server',
            organizations: [{ label: 'foo', value: 'foo' }],
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
          requireMFAType: requireMFATypeOptionsMap.get(false),
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
          createHostUserMode: createHostUserModeOptionsMap.get(''),
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
          createDBUserMode: createDBUserModeOptionsMap.get(''),
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
          defaultSessionRecordingMode: sessionRecordingModeOptionsMap.get(''),
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
          sshSessionRecordingMode: sessionRecordingModeOptionsMap.get(''),
        },
      },
    },

    {
      name: 'unsupported value in spec.options.ssh_port_forwarding',
      role: {
        ...minRole,
        spec: {
          ...minRole.spec,
          options: optionsWithDefaults({
            ssh_port_forwarding: {},
          }),
        },
      },
      model: roleModelWithReset,
    },
  ])(
    'requires reset because of $name',
    ({ role, model = roleModelWithReset }) => {
      expect(roleToRoleEditorModel(role)).toEqual(model);
    }
  );

  test('unsupported version requires reset', () => {
    expect(
      roleToRoleEditorModel({ ...minimalRole(), version: 'v1' as RoleVersion })
    ).toEqual({
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
        version: roleVersionOptionsMap.get(defaultRoleVersion),
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
        version: roleVersionOptionsMap.get(defaultRoleVersion),
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
            kubernetes_users: ['alice', 'bob'],
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
              kind: kubernetesResourceKindOptionsMap.get('pod'),
              name: 'some-pod',
              namespace: '*',
              verbs: [
                kubernetesVerbOptionsMap.get('get'),
                kubernetesVerbOptionsMap.get('update'),
              ],
              roleVersion: defaultRoleVersion,
            },
            {
              id: expect.any(String),
              kind: kubernetesResourceKindOptionsMap.get('kube_node'),
              name: 'some-node',
              namespace: '',
              verbs: [],
              roleVersion: defaultRoleVersion,
            },
          ],
          users: [
            { label: 'alice', value: 'alice' },
            { label: 'bob', value: 'bob' },
          ],
          roleVersion: defaultRoleVersion,
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
              {
                resources: [ResourceKind.Session],
                verbs: ['read', 'list'],
                where: 'contains(session.participants, user.metadata.name)',
              },
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
            resourceKindOptionsMap.get(ResourceKind.User),
            resourceKindOptionsMap.get(ResourceKind.DatabaseService),
          ],
          verbs: [verbOptionsMap.get('read'), verbOptionsMap.get('list')],
          where: '',
        },
        {
          id: expect.any(String),
          resources: [resourceKindOptionsMap.get(ResourceKind.Lock)],
          verbs: [verbOptionsMap.get('create')],
          where: '',
        },
        {
          id: expect.any(String),
          resources: [resourceKindOptionsMap.get(ResourceKind.Session)],
          verbs: [verbOptionsMap.get('read'), verbOptionsMap.get('list')],
          where: 'contains(session.participants, user.metadata.name)',
        },
      ],
    } as RoleEditorModel);
  });

  test('multiple github_permissions', () => {
    expect(
      roleToRoleEditorModel({
        ...minimalRole(),
        spec: {
          ...minimalRole().spec,
          allow: {
            ...minimalRole().spec.allow,
            github_permissions: [{ orgs: ['foo'] }, { orgs: ['bar'] }],
          },
        },
      })
    ).toEqual({
      ...minimalRoleModel(),
      resources: [
        {
          kind: 'git_server',
          organizations: [
            { label: 'foo', value: 'foo' },
            { label: 'bar', value: 'bar' },
          ],
        },
      ],
    } as RoleEditorModel);
  });

  it.each(['access', 'editor', 'auditor'])(
    'supports the preset "%s" role',
    roleName => {
      const { requiresReset } = roleToRoleEditorModel(presetRoles[roleName]);
      expect(requiresReset).toBe(false);
    }
  );
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
          version: roleVersionOptionsMap.get(RoleVersion.V5),
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
      version: 'v5',
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
                kind: kubernetesResourceKindOptionsMap.get('pod'),
                name: 'some-pod',
                namespace: '*',
                verbs: [
                  kubernetesVerbOptionsMap.get('get'),
                  kubernetesVerbOptionsMap.get('update'),
                ],
                roleVersion: defaultRoleVersion,
              },
              {
                id: 'dummy-id-2',
                kind: kubernetesResourceKindOptionsMap.get('kube_node'),
                name: 'some-node',
                namespace: '',
                verbs: [],
                roleVersion: defaultRoleVersion,
              },
            ],
            users: [
              { label: 'alice', value: 'alice' },
              { label: 'bob', value: 'bob' },
            ],
            roleVersion: defaultRoleVersion,
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
          kubernetes_users: ['alice', 'bob'],
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
              resourceKindOptionsMap.get(ResourceKind.User),
              resourceKindOptionsMap.get(ResourceKind.DatabaseService),
            ],
            verbs: [verbOptionsMap.get('read'), verbOptionsMap.get('list')],
            where: '',
          },
          {
            id: 'dummy-id-2',
            resources: [resourceKindOptionsMap.get(ResourceKind.Lock)],
            verbs: [verbOptionsMap.get('create')],
            where: '',
          },
          {
            id: expect.any(String),
            resources: [resourceKindOptionsMap.get(ResourceKind.Session)],
            verbs: [verbOptionsMap.get('read'), verbOptionsMap.get('list')],
            where: 'contains(session.participants, user.metadata.name)',
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
            {
              resources: ['session'],
              verbs: ['read', 'list'],
              where: 'contains(session.participants, user.metadata.name)',
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
