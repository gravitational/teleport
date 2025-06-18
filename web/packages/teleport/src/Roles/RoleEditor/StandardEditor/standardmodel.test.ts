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
  MCPPermissions,
  RequireMFAType,
  ResourceKind,
  Role,
  RoleOptions,
  RoleVersion,
  Rule,
  SessionRecordingMode,
  SSHPortForwarding,
  Verb,
} from 'teleport/services/resources';

import presetRoles from '../../../../../../../gen/preset-roles.json';
import {
  ConversionErrorType,
  simpleConversionErrors,
  unsupportedValueWithReplacement,
} from './errors';
import {
  allowedVerbsForResourceKinds,
  createDBUserModeOptionsMap,
  createHostUserModeOptionsMap,
  defaultRoleVersion,
  gitHubOrganizationsToModel,
  KubernetesAccess,
  kubernetesResourceKindOptionsMapV7,
  kubernetesVerbOptionsMap,
  labelsModelToLabels,
  labelsToModel,
  mcpToolsToModel,
  portForwardingOptionsToModel,
  requireMFATypeOptionsMap,
  resourceKindOptionsMap,
  RoleEditorModel,
  roleEditorModelToRole,
  roleToRoleEditorModel,
  roleVersionOptionsMap,
  sessionRecordingModeOptionsMap,
  sshPortForwardingModeOptionsMap,
} from './standardmodel';
import { withDefaults } from './withDefaults';

const minimalRole = (roleVersion = defaultRoleVersion) =>
  withDefaults({ metadata: { name: 'foobar' } }, roleVersion);

const minimalRoleModel = (
  roleVersion = defaultRoleVersion
): RoleEditorModel => ({
  metadata: {
    name: 'foobar',
    nameCollision: false,
    labels: [],
    version: roleVersionOptionsMap.get(roleVersion),
  },
  resources: [],
  rules: [],
  requiresReset: false,
  conversionErrors: [],
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

test('conversion error utilities', () => {
  expect(
    simpleConversionErrors(ConversionErrorType.UnsupportedField, ['foo', 'bar'])
  ).toEqual([
    {
      type: ConversionErrorType.UnsupportedField,
      path: 'foo',
    },
    {
      type: ConversionErrorType.UnsupportedField,
      path: 'bar',
    },
  ]);

  expect(
    simpleConversionErrors(ConversionErrorType.UnsupportedValue, ['foo'])
  ).toEqual([
    {
      type: ConversionErrorType.UnsupportedValue,
      path: 'foo',
    },
  ]);

  expect(unsupportedValueWithReplacement('foo', { bar: 123 })).toEqual({
    type: ConversionErrorType.UnsupportedValueWithReplacement,
    path: 'foo',
    replacement: '{"bar":123}',
  });
});

// These tests make sure that role to model and model to role conversions are
// symmetrical in typical cases.
describe.each<{ name: string; role: Role; model: RoleEditorModel }>([
  { name: 'minimal role', role: minimalRole(), model: minimalRoleModel() },

  {
    name: 'metadata',
    role: {
      ...minimalRole(RoleVersion.V6),
      metadata: {
        name: 'role-name',
        description: 'role-description',
        labels: { foo: 'bar' },
      },
    },
    model: {
      ...minimalRoleModel(RoleVersion.V6),
      metadata: {
        name: 'role-name',
        nameCollision: false,
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
          hideValidationErrors: false,
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
          mcpTools: [],
          hideValidationErrors: false,
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
          hideValidationErrors: false,
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
          hideValidationErrors: false,
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
          hideValidationErrors: false,
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

test.each<
  {
    name: string;
    expected: ReturnType<typeof portForwardingOptionsToModel>;
  } & Pick<RoleOptions, 'ssh_port_forwarding' | 'port_forwarding'>
>([
  {
    name: 'unspecified',
    expected: { model: '', conversionErrors: [] },
  },
  {
    name: 'none',
    ssh_port_forwarding: {
      local: { enabled: false },
      remote: { enabled: false },
    },
    expected: { model: 'none', conversionErrors: [] },
  },
  {
    name: 'local-only',
    ssh_port_forwarding: {
      local: { enabled: true },
      remote: { enabled: false },
    },
    expected: {
      model: 'local-only',
      conversionErrors: [],
    },
  },
  {
    name: 'remote-only',
    ssh_port_forwarding: {
      local: { enabled: false },
      remote: { enabled: true },
    },
    expected: {
      model: 'remote-only',
      conversionErrors: [],
    },
  },
  {
    name: ' local-and-remote',
    ssh_port_forwarding: {
      local: { enabled: true },
      remote: { enabled: true },
    },
    expected: {
      model: 'local-and-remote',
      conversionErrors: [],
    },
  },
  {
    name: 'deprecated-on',
    port_forwarding: true,
    expected: {
      model: 'deprecated-on',
      conversionErrors: [],
    },
  },
  {
    name: 'deprecated-off',
    port_forwarding: false,
    expected: {
      model: 'deprecated-off',
      conversionErrors: [],
    },
  },
  {
    name: 'local-and-remote (overriding deprecated even if specified)',
    ssh_port_forwarding: {
      local: { enabled: true },
      remote: { enabled: true },
    },
    port_forwarding: false,
    expected: {
      model: 'local-and-remote',
      conversionErrors: [],
    },
  },
  {
    name: 'an empty port forwarding object',
    ssh_port_forwarding: {},
    expected: {
      model: '',
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedValue,
          path: 'spec.options.ssh_port_forwarding',
        },
      ],
    },
  },
  {
    name: 'empty local and remote objects',
    ssh_port_forwarding: { local: {}, remote: {} },
    expected: {
      model: '',
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedValue,
          path: 'spec.options.ssh_port_forwarding',
        },
      ],
    },
  },
  {
    name: 'unknown fields',
    ssh_port_forwarding: {
      local: { enabled: true, localFoo: 'bar' },
      remote: { enabled: true, remoteFoo: 'bar' },
      foo: 'bar',
    } as SSHPortForwarding,
    expected: {
      model: 'local-and-remote',
      conversionErrors: simpleConversionErrors(
        ConversionErrorType.UnsupportedField,
        [
          'spec.options.ssh_port_forwarding.foo',
          'spec.options.ssh_port_forwarding.local.localFoo',
          'spec.options.ssh_port_forwarding.remote.remoteFoo',
        ]
      ),
    },
  },
])(
  'portForwardingOptionsToModel(): $name',
  ({ ssh_port_forwarding, port_forwarding, expected }) => {
    expect(
      portForwardingOptionsToModel(
        { ssh_port_forwarding, port_forwarding },
        'spec.options'
      )
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
    expected: { model: [], conversionErrors: [] },
  },
  {
    name: 'some organizations',
    permissions: [{ orgs: ['foo', 'bar'] }],
    expected: {
      model: [
        { label: 'foo', value: 'foo' },
        { label: 'bar', value: 'bar' },
      ],
      conversionErrors: [],
    },
  },
  {
    name: 'empty permissions object',
    permissions: [{}],
    expected: {
      model: [],
      conversionErrors: [],
    },
  },
  {
    name: 'empty organizations array',
    permissions: [{ orgs: [] }],
    expected: { model: [], conversionErrors: [] },
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
      conversionErrors: [],
    },
  },
  {
    name: 'invalid fields',
    permissions: [{ orgs: ['foo'], unknownField: 123 } as GitHubPermission],
    expected: {
      model: [{ label: 'foo', value: 'foo' }],
      conversionErrors: simpleConversionErrors(
        ConversionErrorType.UnsupportedField,
        ['spec.allow.github_permissions[0].unknownField']
      ),
    },
  },
])('gitHubOrganizationsToModel(): $name', ({ permissions, expected }) => {
  expect(
    gitHubOrganizationsToModel(permissions, 'spec.allow.github_permissions')
  ).toEqual(expected);
});

test.each<{
  name: string;
  permissions: MCPPermissions;
  expected: ReturnType<typeof mcpToolsToModel>;
}>([
  {
    name: 'empty permissions',
    permissions: {},
    expected: { model: [], conversionErrors: [] },
  },
  {
    name: 'empty tools',
    permissions: { tools: [] },
    expected: { model: [], conversionErrors: [] },
  },
  {
    name: 'some tools',
    permissions: { tools: ['foo1', 'foo2'] },
    expected: {
      model: ['foo1', 'foo2'],
      conversionErrors: [],
    },
  },
  {
    name: 'invalid fields',
    permissions: { tools: ['foo'], unknownField: 123 } as MCPPermissions,
    expected: {
      model: ['foo'],
      conversionErrors: simpleConversionErrors(
        ConversionErrorType.UnsupportedField,
        ['spec.allow.mcp.unknownField']
      ),
    },
  },
])('mcpToolsToModel(): $name', ({ permissions, expected }) => {
  expect(mcpToolsToModel(permissions, 'spec.allow.mcp')).toEqual(expected);
});

describe('roleToRoleEditorModel', () => {
  const minRole = minimalRole();
  const minRoleModel = minimalRoleModel();
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
    hideValidationErrors: false,
  });

  test('unknown and invalid fields are reported as conversion errors', () => {
    const role = {
      ...minRole,
      unknown1: 123,
      unknown2: 234,
      metadata: { name: 'foobar', metadataUnknown: 123 },
      spec: {
        ...minRole.spec,
        specUnknown: 123,
        allow: {
          ...minRole.spec.allow,
          allowUnknown: 123,
          kubernetes_resources: [
            { kind: 'jobs', resUnknown: 123 } as KubernetesResource,
            {
              kind: '*',
              verbs: ['illegal', 'get'],
            } as unknown as KubernetesResource,
          ],
          github_permissions: [
            { orgs: ['foo'], gitHubUnknown: 123 } as GitHubPermission,
          ],
          rules: [
            { ruleUnknown: 123 } as Rule,
            { verbs: ['create', 'illegal'] } as Rule,
            // These are all technically valid, but you can't rotate a node
            // (though it might be fun).
            { resources: ['node'], verbs: ['read', 'rotate'] } as Rule,
            // Other verbs are unsupported along with a wildcard.
            { resources: ['node'], verbs: ['*', 'read'] } as Rule,
          ],
        },
        deny: { ...minRole.spec.deny, denyUnknown: 123 },
        options: {
          ...minRole.spec.options,
          optionsUnknown: 123,
          record_session: {
            ...minRole.spec.options.record_session,
            recordSessionUnknown: 123,
            default: 'bogus' as SessionRecordingMode,
            ssh: 'bogus' as SessionRecordingMode,
          },
          require_session_mfa: 'bogus' as RequireMFAType,
          create_host_user_mode: 'bogus' as CreateHostUserMode,
          create_db_user_mode: 'bogus' as CreateDBUserMode,
          ssh_port_forwarding: {},
          cert_format: 'unsupported-format',
          enhanced_recording: [],
          pin_source_ip: true,
          ssh_file_copy: false,
        },
      },
    } as Role;
    const model: RoleEditorModel = {
      ...minRoleModel,
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedField,
          errors: simpleConversionErrors(ConversionErrorType.UnsupportedField, [
            'metadata.metadataUnknown',
            'spec.allow.allowUnknown',
            'spec.allow.github_permissions[0].gitHubUnknown',
            'spec.allow.kubernetes_resources[0].resUnknown',
            'spec.allow.rules[0].ruleUnknown',
            'spec.deny.denyUnknown',
            'spec.options.optionsUnknown',
            'spec.options.record_session.recordSessionUnknown',
            'spec.specUnknown',
            'unknown1',
            'unknown2',
          ]),
        },
        {
          type: ConversionErrorType.UnsupportedValue,
          errors: simpleConversionErrors(ConversionErrorType.UnsupportedValue, [
            'spec.allow.kubernetes_resources[1].verbs[0]',
            'spec.allow.rules[1].verbs[1]',
            'spec.allow.rules[2].verbs[1]',
            'spec.options.ssh_port_forwarding',
          ]),
        },
        {
          type: ConversionErrorType.UnsupportedValueWithReplacement,
          errors: [
            unsupportedValueWithReplacement('spec.allow.rules[3].verbs', ['*']),
            unsupportedValueWithReplacement(
              'spec.options.cert_format',
              'standard'
            ),
            unsupportedValueWithReplacement(
              'spec.options.create_db_user_mode',
              ''
            ),
            unsupportedValueWithReplacement(
              'spec.options.create_host_user_mode',
              ''
            ),
            unsupportedValueWithReplacement('spec.options.enhanced_recording', [
              'command',
              'network',
            ]),
            unsupportedValueWithReplacement(
              'spec.options.pin_source_ip',
              false
            ),
            unsupportedValueWithReplacement(
              'spec.options.record_session.default',
              ''
            ),
            unsupportedValueWithReplacement(
              'spec.options.record_session.ssh',
              ''
            ),
            unsupportedValueWithReplacement(
              'spec.options.require_session_mfa',
              false
            ),
            unsupportedValueWithReplacement('spec.options.ssh_file_copy', true),
          ],
        },
      ],
      requiresReset: true,
      resources: [
        {
          ...newKubeClusterResourceAccess(),
          resources: [
            expect.objectContaining({
              kind: { value: 'jobs', label: 'jobs' },
            }),
            expect.objectContaining({
              verbs: [kubernetesVerbOptionsMap.get('get')],
            }),
          ],
        },
        {
          kind: 'git_server',
          organizations: [{ label: 'foo', value: 'foo' }],
          hideValidationErrors: false,
        },
      ],
      rules: [
        expect.any(Object),
        expect.objectContaining({
          allVerbs: false,
          verbs: [
            { verb: 'read', checked: false },
            { verb: 'list', checked: false },
            { verb: 'create', checked: true },
            { verb: 'update', checked: false },
            { verb: 'delete', checked: false },
          ],
        }),
        expect.objectContaining({
          resources: [{ label: 'node', value: 'node' }],
          allVerbs: false,
          verbs: [
            { verb: 'read', checked: true },
            { verb: 'list', checked: false },
            { verb: 'create', checked: false },
            { verb: 'update', checked: false },
            { verb: 'delete', checked: false },
          ],
        }),
        expect.objectContaining({
          resources: [{ label: 'node', value: 'node' }],
          allVerbs: true,
          verbs: [
            { verb: 'read', checked: true },
            { verb: 'list', checked: true },
            { verb: 'create', checked: true },
            { verb: 'update', checked: true },
            { verb: 'delete', checked: true },
          ],
        }),
      ],
      options: {
        ...roleModelWithReset.options,
        defaultSessionRecordingMode: sessionRecordingModeOptionsMap.get(''),
      },
    };

    expect(roleToRoleEditorModel(role)).toEqual(model);
  });

  test('unsupported version requires reset', () => {
    expect(
      roleToRoleEditorModel({ ...minimalRole(), version: 'v1' as RoleVersion })
    ).toEqual({
      ...minimalRoleModel(),
      requiresReset: true,
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedValueWithReplacement,
          errors: [
            {
              type: ConversionErrorType.UnsupportedValueWithReplacement,
              path: 'version',
              replacement: '"v8"',
            },
          ],
        },
      ],
    } as RoleEditorModel);
  });

  test('support custom resource', () => {
    expect(
      roleToRoleEditorModel({
        ...minRole,
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            kubernetes_resources: [
              { kind: 'unknown', api_group: 'group.example.com' },
              { kind: 'jobs', api_group: '*' },
            ],
          },
        },
      })
    ).toEqual({
      ...minRoleModel,
      resources: [
        {
          ...newKubeClusterResourceAccess(),
          resources: [
            expect.objectContaining({
              kind: { value: 'unknown', label: 'unknown' },
              apiGroup: 'group.example.com',
            }),
            expect.objectContaining({
              kind: { value: 'jobs', label: 'jobs' },
              apiGroup: '*',
            }),
          ],
        },
      ],
    });
  });

  test('not support custom resource', () => {
    expect(
      roleToRoleEditorModel({
        ...minimalRole(RoleVersion.V7),
        spec: {
          ...minimalRole(RoleVersion.V7).spec,
          allow: {
            ...minimalRole(RoleVersion.V7).spec.allow,
            kubernetes_resources: [
              { kind: 'unknown', api_group: 'group.example.com' },
              { kind: 'jobs', api_group: '*' },
            ],
          },
          options: {
            ...minimalRole(RoleVersion.V7).spec.options,
            idp: { saml: { enabled: true } },
          },
        },
      })
    ).toEqual({
      ...minimalRoleModel(RoleVersion.V7),
      requiresReset: true,
      resources: [],
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedValue,
          errors: [
            {
              type: ConversionErrorType.UnsupportedValue,
              path: 'spec.allow.kubernetes_resources[0].kind',
            },
            {
              type: ConversionErrorType.UnsupportedValue,
              path: 'spec.allow.kubernetes_resources[1].kind',
            },
          ],
        },
      ],
    });
  });

  test('v8 not support v7 kind', () => {
    expect(
      roleToRoleEditorModel({
        ...minimalRole(RoleVersion.V8),
        spec: {
          ...minRole.spec,
          allow: {
            ...minRole.spec.allow,
            kubernetes_resources: [{ kind: 'job', api_group: '*' }],
          },
        },
      })
    ).toEqual({
      ...minimalRoleModel(RoleVersion.V8),
      requiresReset: true,
      resources: [
        {
          ...newKubeClusterResourceAccess(),
          resources: [
            expect.objectContaining({ kind: { value: 'jobs', label: 'jobs' } }),
          ],
        },
      ],
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedValueWithReplacement,
          errors: [
            {
              type: ConversionErrorType.UnsupportedValueWithReplacement,
              path: 'spec.allow.kubernetes_resources[0].kind',
              replacement: '"jobs"',
            },
          ],
        },
      ],
    });
  });

  it('revision change requires reset', () => {
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
      metadata: expect.objectContaining({
        // We need to preserve the original revision.
        revision: originalRev,
      }),
      requiresReset: true,
      conversionErrors: [
        {
          type: ConversionErrorType.UnsupportedChange,
          errors: [
            {
              type: ConversionErrorType.UnsupportedChange,
              path: 'metadata.revision',
            },
          ],
        },
      ],
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
                kind: 'pods',
                name: 'some-pod',
                namespace: '*',
                verbs: ['get', 'update'],
              },
              {
                // No namespace required for cluster-wide resources.
                kind: 'nodes',
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
              kind: { value: 'pods', label: 'pods' },
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
              kind: { value: 'nodes', label: 'nodes' },
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
          hideValidationErrors: false,
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
          mcpTools: [],
          hideValidationErrors: false,
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
              { resources: [ResourceKind.Lock], verbs: ['*'] },
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
          allVerbs: false,
          verbs: [
            { verb: 'read', checked: true },
            { verb: 'list', checked: true },
            { verb: 'create', checked: false },
            { verb: 'update', checked: false },
            { verb: 'delete', checked: false },
          ],
          where: '',
          hideValidationErrors: false,
        },
        {
          id: expect.any(String),
          resources: [resourceKindOptionsMap.get(ResourceKind.Lock)],
          allVerbs: true,
          verbs: [
            { verb: 'read', checked: true },
            { verb: 'list', checked: true },
            { verb: 'create', checked: true },
            { verb: 'update', checked: true },
            { verb: 'delete', checked: true },
          ],
          where: '',
          hideValidationErrors: false,
        },
        {
          id: expect.any(String),
          resources: [resourceKindOptionsMap.get(ResourceKind.Session)],
          allVerbs: false,
          verbs: [
            { verb: 'read', checked: true },
            { verb: 'list', checked: true },
            { verb: 'create', checked: false },
            { verb: 'update', checked: false },
            { verb: 'delete', checked: false },
          ],
          where: 'contains(session.participants, user.metadata.name)',
          hideValidationErrors: false,
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
          hideValidationErrors: false,
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
        ...minimalRoleModel(RoleVersion.V5),
        metadata: {
          name: 'dog-walker',
          nameCollision: false,
          description: 'walks dogs',
          revision: 'e2a3ccf8-09b9-4d97-8e47-6dbe3d53c0e5',
          labels: [{ name: 'kind', value: 'occupation' }],
          version: roleVersionOptionsMap.get(RoleVersion.V5),
        },
      })
    ).toEqual({
      ...minimalRole(RoleVersion.V5),
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
                kind: kubernetesResourceKindOptionsMapV7.get('pod'),
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
                kind: kubernetesResourceKindOptionsMapV7.get('kube_node'),
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
            hideValidationErrors: false,
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
            allVerbs: false,
            verbs: [
              { verb: 'read', checked: true },
              { verb: 'list', checked: true },
              { verb: 'create', checked: false },
              { verb: 'update', checked: false },
              { verb: 'delete', checked: false },
            ],
            where: '',
            hideValidationErrors: false,
          },
          {
            id: 'dummy-id-2',
            resources: [resourceKindOptionsMap.get(ResourceKind.Lock)],
            allVerbs: true,
            verbs: [
              { verb: 'read', checked: false },
              { verb: 'list', checked: false },
              // Set one of these to `true` to make sure it's ignored and
              // overridden by `allVerbs`. Shouldn't happen, but just in case.
              { verb: 'create', checked: true },
              { verb: 'update', checked: false },
              { verb: 'delete', checked: false },
            ],
            where: '',
            hideValidationErrors: false,
          },
          {
            id: 'dummy-id-3',
            resources: [resourceKindOptionsMap.get(ResourceKind.Session)],
            allVerbs: false,
            verbs: [
              { verb: 'read', checked: true },
              { verb: 'list', checked: true },
              { verb: 'create', checked: false },
              { verb: 'update', checked: false },
              { verb: 'delete', checked: false },
            ],
            where: 'contains(session.participants, user.metadata.name)',
            hideValidationErrors: false,
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
            { resources: ['lock'], verbs: ['*'] },
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

test.each<{ kinds: string[]; verbs: Verb[] }>([
  // Typical case
  {
    kinds: [ResourceKind.Node],
    verbs: ['read', 'list', 'create', 'update', 'delete'],
  },
  // Additional verbs
  {
    kinds: [ResourceKind.CertAuthority],
    verbs: [
      'read',
      'list',
      'create',
      'update',
      'delete',
      'readnosecrets',
      'rotate',
    ],
  },
  // Combining verbs from more than one resource kind
  {
    kinds: [ResourceKind.SAMLConnector, ResourceKind.Device],
    verbs: [
      'read',
      'list',
      'create',
      'update',
      'delete',
      'readnosecrets',
      'create_enroll_token',
      'enroll',
    ],
  },
])('allowedVerbsForResourceKinds($kinds)', ({ kinds, verbs }) => {
  expect(allowedVerbsForResourceKinds(kinds)).toEqual(verbs);
});
