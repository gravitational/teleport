import { MinimumRoleVersionSupported } from 'e-teleport/AccessListManagement/GuideEditor/useGuideEditor';
import { optionsWithDefaults } from 'teleport/Roles/RoleEditor/StandardEditor/withDefaults';
import { Role } from 'teleport/services/resources';

import { AppIdentities } from '../resources/app';
import {
  extractAllowRoleConditionsFromRole,
  extractRequiredAppIdentitiesFromRole,
  StandardRoleConditions,
} from './standard';

describe('extractAllowRoleConditionsFromRole', () => {
  test('extracts all fields with values', () => {
    const condition: StandardRoleConditions = {
      app_labels: { env: 'prod' },
      aws_role_arns: ['arn:aws:iam::123:role/admin'],
      azure_identities: ['azure-identity'],
      gcp_service_accounts: ['gcp@project.iam.gserviceaccount.com'],
      mcp: { tools: ['tool1', 'tool2'] },
      db_labels: { db: 'postgres' },
      db_names: ['mydb'],
      db_users: ['dbuser'],
      windows_desktop_labels: { os: 'windows' },
      windows_desktop_logins: ['Administrator'],
      linux_desktop_labels: { os: 'ubuntu' },
      linux_desktop_logins: ['alice'],
      kubernetes_labels: { cluster: 'prod' },
      kubernetes_groups: ['system:masters'],
      kubernetes_users: ['admin'],
      kubernetes_resources: [{ kind: 'pod', name: '*', namespace: '*' }],
      node_labels: { role: 'web' },
      logins: ['root', 'ubuntu'],
      github_permissions: [{ orgs: ['myorg'] }],
    };

    const role = createRole(condition);
    expect(extractAllowRoleConditionsFromRole(role)).toEqual(condition);
  });

  test('returns empty defaults when allow fields are undefined', () => {
    const role = createRole({});

    expect(extractAllowRoleConditionsFromRole(role)).toEqual({
      app_labels: {},
      aws_role_arns: [],
      azure_identities: [],
      gcp_service_accounts: [],
      mcp: { tools: [] },
      db_labels: {},
      db_names: [],
      db_users: [],
      windows_desktop_labels: {},
      windows_desktop_logins: [],
      linux_desktop_labels: {},
      linux_desktop_logins: [],
      kubernetes_labels: {},
      kubernetes_groups: [],
      kubernetes_users: [],
      kubernetes_resources: [],
      node_labels: {},
      logins: [],
      github_permissions: [],
    });
  });

  test('returns empty defaults when allow is missing', () => {
    const role = createRole({});
    delete role.spec.allow;

    expect(extractAllowRoleConditionsFromRole(role)).toEqual({
      app_labels: {},
      aws_role_arns: [],
      azure_identities: [],
      gcp_service_accounts: [],
      mcp: { tools: [] },
      db_labels: {},
      db_names: [],
      db_users: [],
      windows_desktop_labels: {},
      windows_desktop_logins: [],
      linux_desktop_labels: {},
      linux_desktop_logins: [],
      kubernetes_labels: {},
      kubernetes_groups: [],
      kubernetes_users: [],
      kubernetes_resources: [],
      node_labels: {},
      logins: [],
      github_permissions: [],
    });
  });
});

describe('extractRequiredAppIdentitiesFromRole', () => {
  test('returns empty arrays for fields that have values in the role', () => {
    const condition: AppIdentities = {
      aws_role_arns: ['arn:aws:iam::123:role/admin'],
      azure_identities: ['azure-identity'],
      gcp_service_accounts: ['gcp@project.iam.gserviceaccount.com'],
      mcp: { tools: ['tool1'] },
    };
    const role = createRole(condition);

    expect(extractRequiredAppIdentitiesFromRole(role)).toEqual({
      aws_role_arns: [],
      azure_identities: [],
      gcp_service_accounts: [],
      mcp: { tools: [] },
      allPagesFetched: false,
    });
  });

  test('returns undefined for fields that are undefined in the role', () => {
    const role = createRole({});

    expect(extractRequiredAppIdentitiesFromRole(role)).toEqual({
      aws_role_arns: undefined,
      azure_identities: undefined,
      gcp_service_accounts: undefined,
      mcp: undefined,
      allPagesFetched: false,
    });
  });

  test('returns undefined fields when allow is missing', () => {
    const role = createRole({});
    delete role.spec.allow;

    expect(extractRequiredAppIdentitiesFromRole(role)).toEqual({
      aws_role_arns: undefined,
      azure_identities: undefined,
      gcp_service_accounts: undefined,
      mcp: undefined,
      allPagesFetched: false,
    });
  });

  test('returns undefined for fields that are empty arrays in the role', () => {
    const role = createRole({
      aws_role_arns: [],
      azure_identities: [],
      gcp_service_accounts: [],
      mcp: { tools: [] },
    });

    expect(extractRequiredAppIdentitiesFromRole(role)).toEqual({
      aws_role_arns: undefined,
      azure_identities: undefined,
      gcp_service_accounts: undefined,
      mcp: undefined,
      allPagesFetched: false,
    });
  });

  test('handles mixed fields with some having values and some empty', () => {
    const role = createRole({
      aws_role_arns: ['arn:aws:iam::123:role/admin'],
      azure_identities: [],
      gcp_service_accounts: undefined,
      mcp: { tools: ['tool1', 'tool2'] },
    });

    expect(extractRequiredAppIdentitiesFromRole(role)).toEqual({
      aws_role_arns: [],
      azure_identities: undefined,
      gcp_service_accounts: undefined,
      mcp: { tools: [] },
      allPagesFetched: false,
    });
  });
});

const createRole = (allow: Partial<Role['spec']['allow']>): Role => ({
  kind: 'role',
  version: MinimumRoleVersionSupported,
  metadata: { name: 'test-role' },
  spec: {
    allow,
    deny: {},
    options: optionsWithDefaults(MinimumRoleVersionSupported),
  },
});
