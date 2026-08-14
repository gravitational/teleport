import { defaultRoleVersion } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { optionsWithDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';
import { Role, RoleVersion } from 'teleport/services/resources';

import { MinimumRoleVersionSupported } from '../../useGuideEditor';
import { AwsIcAppLabel, RequiredRoleConditions } from '../role/conditions';

export const testAccessListId = 'ABCD';

export const internalAccessListPresetLabelKey =
  'teleport.internal/access-list-preset';

const awsIcMetadata = {
  name: `access-awsic-acl-preset-${testAccessListId}`,
  labels: { [internalAccessListPresetLabelKey]: testAccessListId },
};

const standardMetadata = {
  name: `access-standard-acl-preset-${testAccessListId}`,
  labels: { [internalAccessListPresetLabelKey]: testAccessListId },
};

export const awsIcRole: Role = {
  kind: 'role',
  version: MinimumRoleVersionSupported,
  metadata: awsIcMetadata,
  spec: {
    allow: {
      app_labels: AwsIcAppLabel,
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
      ],
    },
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleFullyFleshedOut: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: awsIcMetadata,
  spec: {
    allow: {
      // supported fields
      app_labels: AwsIcAppLabel,
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
        { account: 'account2', permission_set: 'permission-set2' },
      ],

      // unsupported fields can be "defined" but empty
      db_labels: {},
      kubernetes_labels: {},
      windows_desktop_labels: {},
      linux_desktop_labels: {},
      node_labels: {},
      logins: [],
      windows_desktop_logins: [],
      linux_desktop_logins: [],
      azure_identities: [],
      mcp: { tools: [] },
      gcp_service_accounts: [],
      aws_role_arns: [],
      db_names: [],
      db_users: [],
      kubernetes_groups: [],
      kubernetes_resources: [],
      kubernetes_users: [],
      github_permissions: [],
      db_service_labels: {},
      rules: [],
      db_roles: [],
    } as RequiredRoleConditions,
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleWithoutAccountAssignments: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: awsIcMetadata,
  spec: {
    allow: { app_labels: AwsIcAppLabel },
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleWithoutLabels: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: awsIcMetadata,
  spec: {
    allow: {
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
      ],
    },
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleWithDeny: Role = {
  kind: 'role',
  version: MinimumRoleVersionSupported,
  metadata: awsIcMetadata,
  spec: {
    allow: {
      app_labels: AwsIcAppLabel,
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
      ],
    },
    deny: { app_labels: AwsIcAppLabel },
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleWithUnknownFields: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: awsIcMetadata,
  spec: {
    allow: {
      app_labels: AwsIcAppLabel,
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
      ],
      db_labels: { env: 'prod' },
    },
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleEmptyAccess: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: awsIcMetadata,
  spec: {
    allow: {},
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const awsIcRoleWithUnsupportedVersion: Role = {
  kind: 'role',
  version: RoleVersion.V3,
  metadata: awsIcMetadata,
  spec: {
    allow: {
      app_labels: AwsIcAppLabel,
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
      ],
    },
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const standardRole: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: standardMetadata,
  spec: {
    allow: { app_labels: { env: 'prod' } },
  } as any, // to test undefined deny and option fields
};

export const standardRoleFullyFleshedOut: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: standardMetadata,
  spec: {
    allow: {
      // supported fields
      app_labels: { env: 'prod' },
      db_labels: { env: 'prod' },
      kubernetes_labels: { env: 'prod' },
      windows_desktop_labels: { env: 'prod' },
      linux_desktop_labels: { env: 'prod' },
      node_labels: { env: 'prod' },

      logins: ['abc'],
      windows_desktop_logins: ['abc'],
      linux_desktop_logins: ['abc'],
      azure_identities: ['abc'],
      mcp: { tools: ['tool'] },
      gcp_service_accounts: ['abc'],
      aws_role_arns: ['arn'],
      db_names: ['name'],
      db_users: ['user'],
      kubernetes_groups: ['group'],
      kubernetes_resources: [{}],
      kubernetes_users: ['user'],
      github_permissions: ['github'],

      // unsupported fields can be "defined" but empty
      db_service_labels: {},
      rules: [],
      account_assignments: [],
      db_roles: [],
    } as RequiredRoleConditions,
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const standardRoleWithDeny: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: standardMetadata,
  spec: {
    allow: { app_labels: { env: 'prod' } },
    deny: { app_labels: { env: 'prod' } },
    options: optionsWithDefaults(defaultRoleVersion),
  },
};

export const standardRoleWithUnknownFields: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: standardMetadata,
  spec: {
    allow: {
      app_labels: { env: 'prod' },
      account_assignments: [
        { account: 'account', permission_set: 'permission-set' },
      ],
    },
    options: optionsWithDefaults(defaultRoleVersion),
    deny: {},
  },
};

export const standardRoleEmptyAccess: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: standardMetadata,
  spec: {
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  } as any, // to test undefined allow field
};

export const standardRoleWithUnsupportedVersion: Role = {
  kind: 'role',
  version: RoleVersion.V3,
  metadata: standardMetadata,
  spec: {
    allow: {
      app_labels: { env: 'prod' },
    },
    deny: {},
    options: optionsWithDefaults(defaultRoleVersion),
  },
};
