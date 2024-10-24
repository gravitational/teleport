import { Box, Text } from 'design';
import { Info } from 'design/Alert';

import type {
  AwsIcAccounts,
  AwsIcPermissionSets,
  AwsIcGroupsWithAssignment,
} from 'e-teleport/services/plugins/types';

export const DevNoteEnroll = () => (
  <Info>
    <Box>
      <Text typography="body3">
        Devs: use the following values to proceed with the enrollment.
        <OidcNote />
        <b>Default users:</b> Pick at least one user
        <br />
        <b>Upload file:</b> Pick xml file from your machine
        <br />
        <b>SCIM credential:</b> Enter valid HTTPs URL and a random string
        <br />
      </Text>
    </Box>
  </Info>
);

export const DevNoteOidc = () => (
  <Info>
    <Text typography="body3">
      Devs: use the following values to get to the Step 3 and click save button
      to view error
    </Text>
    <OidcNote />
  </Info>
);

const OidcNote = () => (
  <Text typography="body3">
    <b>Instance region:</b> ca-central-1
    <br />
    <b>Instance ARN:</b> arn:aws:sso:::instance/ssoins-8004ee88884dd26a
    <br />
    <b>Intgration name:</b> new
    <br />
    <b>Role ARN:</b> arn:aws:iam::026090554232:role/new
    <br />
  </Text>
);

export const integrationsResponse = {
  items: [
    {
      name: 'new-integration',
      subKind: 'aws-oidc',
      awsoidc: {
        roleArn: 'arn:aws:iam::026090554232:role/new-integration',
      },
      origin: '',
    },
    {
      name: 'test',
      subKind: 'aws-oidc',
      awsoidc: {
        roleArn: 'arn:aws:iam::026090554232:role/test',
      },
      origin: '',
    },
  ],
  nextKey: '',
};

export const users = [
  { name: 'access-user', roles: ['access'], authType: 'local' },
  { name: 'editor-user', roles: ['editor'], authType: 'local' },
  { name: 'auditor-user', roles: ['auditor'], authType: 'local' },
];

export const permissionSets: AwsIcPermissionSets[] = [
  {
    name: 'AIOps',
    description: '',
    arn: 'arn:aws:sso:::permissionSet/ssoins-88345234523dfdd26a/ps-93cddc3fasdf99',
  },
  {
    name: 'NetworkAdministrator',
    description: 'Network admin role',
    arn: 'arn:aws:sso:::permissionSet/ssoins-88345234523dfdd26a/ps-93cs09dc3fasdf99',
  },
  {
    name: 'SuperAdmin',
    description: 'Super admin role',
    arn: 'arn:aws:sso:::permissionSet/ssoins-88345234523dfdd26a/ps-93cddc3faasd2299',
  },
];

export const accounts: AwsIcAccounts[] = [
  {
    name: 'dev-account',
    arn: 'arn:aws"organizations::1234567890:account/o-u0adfj/123456789',
    id: '719283048592',
    permissionSets: permissionSets,
  },
  {
    name: 'prod-account',
    arn: 'arn:aws"organizations::1234567890:account/o-u0adfj/123456789',
    id: '719283098752',
    permissionSets: permissionSets,
  },
  {
    name: 'stage-account',
    arn: 'arn:aws"organizations::1234567890:account/o-u0adfj/123456789',
    id: '719283041526',
    permissionSets: permissionSets,
  },
];

export const groupsWithPermissionAssignment: AwsIcGroupsWithAssignment[] = [
  {
    groupname: 'group1',
    assignments: [
      {
        accountName: 'dev-account',
        permissionSetName: 'NetworkAdministrator',
      },
    ],
  },
  {
    groupname: 'group2',
    assignments: [{ accountName: 'dev-account', permissionSetName: 'DevOps' }],
  },
  {
    groupname: 'group3',
    assignments: [
      {
        accountName: 'stage-account',
        permissionSetName: 'SecurityAdministrator',
      },
    ],
  },
];
