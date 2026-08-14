import {
  defaultAwsIcRoleConditions,
  defaultStandardRoleConditions,
} from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/conditions';
import { definableResourceAccessFields } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/listaccess';
import {
  reviewDayOfMonthOpts,
  reviewFrequencyOpts,
} from 'e-teleport/AccessListManagement/Shared/Audit';
import { AccessListMemberKind } from 'e-teleport/services/accessmanagement';

import { Members, Owners, Spec } from '../types';
import { Summary } from './Summary';

export default {
  title: 'TeleportE/AccessLists/Create/Summary',
};

export function ShortTerm() {
  return (
    <Summary
      spec={sampleSpec}
      preset="short-term"
      members={sampleMembers}
      owners={sampleOwners}
      awsIcRoleConditions={defaultAwsIcRoleConditions()}
      standardRoleConditions={{
        ...defaultStandardRoleConditions(),
        app_labels: { env: 'prod' },
        azure_identities: ['az-id-1'],
        db_labels: { env: 'prod' },
        db_names: ['db-name-1'],
      }}
      definedAccessFields={['app_labels', 'db_labels']}
    />
  );
}

export function LongTerm() {
  return (
    <Summary
      spec={sampleSpec}
      preset="long-term"
      members={sampleMembers}
      owners={sampleOwners}
      awsIcRoleConditions={defaultAwsIcRoleConditions()}
      standardRoleConditions={{
        ...defaultStandardRoleConditions(),
        app_labels: { env: 'prod' },
        azure_identities: ['az-id-1'],
        db_labels: { env: 'prod' },
        db_names: ['db-name-1'],
      }}
      definedAccessFields={['app_labels', 'db_labels']}
    />
  );
}

export function Sparse() {
  return (
    <Summary
      spec={{ ...sampleSpec, description: '' }}
      preset="short-term"
      members={{
        ...sampleMembers,
        selectedRolesRequired: [],
        traitLabels: [],
        selectedMembers: [],
      }}
      owners={{ ...sampleOwners, selectedRolesRequired: [], traitLabels: [] }}
      awsIcRoleConditions={defaultAwsIcRoleConditions()}
      standardRoleConditions={defaultStandardRoleConditions()}
      definedAccessFields={[]}
    />
  );
}

export function EverythingDefined() {
  return (
    <Summary
      spec={sampleSpec}
      preset="short-term"
      members={sampleMembers}
      owners={sampleOwners}
      awsIcRoleConditions={{
        ...defaultAwsIcRoleConditions(),
        account: new Map([
          [
            '111111111111',
            new Set([
              'arn:aws:sso:::permissionSet/ssoins-abc/ps-DevAccess',
              'arn:aws:sso:::permissionSet/ssoins-abc/ps-ReadOnly',
            ]),
          ],
          [
            '222222222222',
            new Set(['arn:aws:sso:::permissionSet/ssoins-abc/ps-AdminAccess']),
          ],
        ]),
      }}
      standardRoleConditions={{
        ...defaultStandardRoleConditions(),
        app_labels: { env: 'prod', region: ['us-east-1', 'us-west-2'] },
        aws_role_arns: ['arn:aws:iam::111111111111:role/DevAccess'],
        azure_identities: [
          '/subscriptions/abc/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/dev',
          '/subscriptions/abc/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/prod',
        ],
        gcp_service_accounts: ['dev@my-project.iam.gserviceaccount.com'],
        mcp: { tools: ['list_files', 'read_file', 'write_file'] },

        db_labels: { env: ['prod', 'staging', 'dev'] },
        db_names: ['db-name-1', 'db-name-2'],
        db_users: ['db-user-1', 'db-user-2'],

        github_permissions: [
          { orgs: ['gravitational', 'teleport-org'] },
          { orgs: ['my-org'] },
        ],

        node_labels: { env: ['prod', 'staging'] },
        logins: ['root', 'ubuntu', 'ec2-user'],

        windows_desktop_labels: { os: 'windows-11' },
        windows_desktop_logins: ['Administrator', 'svc-account'],

        kubernetes_labels: { cluster: 'prod-cluster' },
        kubernetes_groups: ['system:masters', 'developers'],
        kubernetes_users: ['kube-admin'],
        kubernetes_resources: [
          {
            kind: 'pod',
            name: '*',
            namespace: 'default',
            verbs: ['get', 'list', 'watch'],
            api_group: '',
          },
          {
            kind: 'deployment',
            name: 'web-*',
            namespace: 'production',
            verbs: ['get', 'list'],
            api_group: 'apps',
          },
        ],
      }}
      definedAccessFields={definableResourceAccessFields}
    />
  );
}

const sampleSpec: Spec = {
  title: 'An Access List Title',
  description: 'Lorem ipsum dolores george washington description',
  reviewFrequency: reviewFrequencyOpts[1],
  reviewDayOfMonth: reviewDayOfMonthOpts[0],
  auditStartDate: undefined,
};

function makeOwner(
  name: string,
  kind: AccessListMemberKind = AccessListMemberKind.User
): Owners['selectedOwners'][number] {
  return {
    label: name,
    value: { name, membershipKind: kind },
  };
}

function makeMember(
  name: string,
  kind: AccessListMemberKind = AccessListMemberKind.User
): Members['selectedMembers'][number] {
  return {
    label: name,
    value: { name, membershipKind: kind },
  };
}

const sampleOwners: Owners = {
  selectedRolesRequired: [
    { label: 'access-admin', value: 'access-admin' },
    { label: 'auditor', value: 'auditor' },
  ],
  eligibleOwners: [],
  selectedOwners: [
    makeOwner('alice'),
    makeOwner('bob'),
    makeOwner('charlie'),
    makeOwner('dana'),
    makeOwner('eve'),
    makeOwner('frank'),
    makeOwner('grace'),
    makeOwner('heidi'),
    makeOwner('ivan'),
    makeOwner('judy'),
    makeOwner('kevin'),
    makeOwner('laura'),
    makeOwner('mallory'),
    makeOwner('nathan'),
    makeOwner('olivia'),
    makeOwner('peggy'),
    makeOwner('platform-admins', AccessListMemberKind.List),
    makeOwner('security-admins', AccessListMemberKind.List),
    makeOwner('sre-leads', AccessListMemberKind.List),
    makeOwner('infra-admins', AccessListMemberKind.List),
  ],
  traitLabels: [
    { name: 'department', value: 'security' },
    { name: 'team', value: 'platform' },
  ],
  traitLookup: {},
};

const sampleMembers: Members = {
  selectedRolesRequired: [
    { label: 'access', value: 'access' },
    { label: 'editor', value: 'editor' },
  ],
  eligibleMembers: [],
  selectedMembers: [
    makeMember('charlie'),
    makeMember('dana'),
    makeMember('engineering-team', AccessListMemberKind.List),
    makeMember('contractors', AccessListMemberKind.List),
  ],
  traitLabels: [
    { name: 'department', value: 'engineering' },
    { name: 'employment_type', value: 'full-time' },
    { name: 'location', value: 'remote' },
  ],
  traitLookup: {},
};
