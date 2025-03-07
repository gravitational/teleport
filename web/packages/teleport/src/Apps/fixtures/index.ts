/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import makeApp from 'teleport/services/apps/makeApps';

export const awsConsoleApp = makeApp({
  name: 'aws-console-1',
  uri: 'https://console.aws.amazon.com/ec2/v2/home',
  publicAddr: 'awsconsole-1.teleport-proxy.com',
  labels: [
    { name: 'aws_account_id', value: 'A1234' },
    { name: 'env', value: 'dev' },
    { name: 'cluster', value: 'two' },
  ],
  description: 'This is an AWS Console app',
  awsConsole: true,
  awsRoles: [
    {
      name: 'role name',
      arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
      display: 'EC2FullAccess',
    },
    {
      name: 'other role name',
      arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
      display: 'ReallyLonReallyLonggggggEC2FullAccess',
    },
    {
      name: 'thisthing',
      arn: 'arn:aws:iam::joe123:role/EC2ReadOnly',
      display: 'EC2ReadOnly',
    },
    ...new Array(20).fill(undefined).map((_, index) => {
      return {
        name: `long-${index}`,
        arc: `arn:aws:iam::${index}`,
        display: `LONG${index}`,
      };
    }),
  ],
  clusterId: 'one',
  fqdn: 'awsconsole-1.com',
});

export const awsIamIcAccountApp = makeApp({
  name: 'aws-iam-ic-account-1',
  uri: 'https://console.aws.amazon.com',
  publicAddr: 'console.aws.amazon.com',
  subKind: 'aws-ic-account',
  labels: [{ name: 'teleport.dev/origin', value: 'aws-identity-center' }],
  description: 'This is an AWS IAM Identity Center account',
  awsConsole: false,
  permissionSets: [
    {
      name: 'Admin perm set',
      arn: 'arn:aws:sso:::permissionSet/Admin',
      display: 'Admin',
    },
    {
      name: 'ReadOnly perm set',
      arn: 'arn:aws:sso:::permissionSet/ReadOnly',
      display: 'ReadOnly',
    },
  ],
  clusterId: 'one',
  fqdn: 'https://console.aws.amazon.com',
});

export const apps = [
  {
    name: 'Jenkins',
    uri: 'https://jenkins.teleport-proxy.com',
    publicAddr: 'jenkins.teleport-proxy.com',
    description: 'This is a Jenkins app',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'jenkins.one',
  },
  {
    name: 'TheOtherOne',
    uri: 'https://jenkins.teleport-proxy.com',
    publicAddr: 'jenkins.teleport-proxy.com',
    description: 'This is a Jenkins app',
    awsConsole: false,
    labels: [{ name: 'icon', value: 'jenkins' }],
    clusterId: 'one',
    fqdn: 'jenkins.two',
  },
  {
    name: 'Grafana',
    uri: 'https://grafana.teleport-proxy.com',
    publicAddr: 'grafana.teleport-proxy.com',
    description: 'This is a Grafana app',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'g.one',
  },
  {
    kind: 'app',
    name: '11llkk2234234',
    description: 'Teleport Okta',
    uri: 'https://dev-1.okta.com/home/dev-1',
    publicAddr: '234.dev-test.teleport',
    fqdn: '234.dev-test.teleport',
    clusterId: 'dev-test.teleport',
    labels: [
      {
        name: 'okta/org',
        value: 'https://dev-test.okta.com',
      },
      {
        name: 'teleport.dev/origin',
        value: 'okta',
      },
    ],
    awsConsole: false,
    friendlyName: 'Teleport Okta',
  },
  {
    name: 'Company Chat',
    uri: 'https://slack.teleport-proxy.com',
    publicAddr: 'slack.teleport-proxy.com',
    description: 'This is the employee slack channel',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'icon', value: 'slack' },
    ],
    clusterId: 'one',
    fqdn: 's.one',
  },
  {
    name: 'saml_app',
    uri: '',
    publicAddr: '',
    description: 'SAML Application',
    awsConsole: false,
    labels: [],
    clusterId: 'one',
    fqdn: '',
    samlApp: true,
    samlAppSSOUrl: '',
  },
  {
    name: 'okta',
    uri: '',
    publicAddr: '',
    description: 'SAML Application',
    awsConsole: false,
    labels: [],
    clusterId: 'one',
    fqdn: '',
    samlApp: true,
    friendlyName: 'Okta Friendly',
    samlAppSSOUrl: '',
  },
  {
    name: 'Mattermost1',
    uri: 'https://mattermost1.teleport-proxy.com',
    publicAddr: 'mattermost.teleport-proxy.com',
    description: 'This is a Mattermost app',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'two' },
    ],
    clusterId: 'one',
    fqdn: 'mattermost.one',
  },
  {
    name: 'TCP',
    uri: 'tcp://some-address',
    publicAddr: '',
    description: 'This is a TCP app',
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
  },
  {
    name: 'Cloud',
    uri: 'cloud://some-address',
    publicAddr: '',
    description: 'This is a Cloud specific app',
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
  },
  {
    name: 'aws-iam-ic-account-2',
    uri: 'https://console.aws.amazon.com',
    publicAddr: 'console.aws.amazon.com',
    subKind: 'aws-ic-account',
    labels: [{ name: 'teleport.dev/origin', value: 'aws-identity-center' }],
    description: 'This is an AWS IAM Identity Center account',
    awsConsole: false,
    permissionSets: [
      {
        name: 'Admin perm set',
        arn: 'arn:aws:sso:::permissionSet/Admin',
        display: 'Admin',
      },
      {
        name: 'ReadOnly perm set',
        arn: 'arn:aws:sso:::permissionSet/ReadOnly',
        display: 'ReadOnly',
      },
    ],
    clusterId: 'one',
    fqdn: 'https://console.aws.amazon.com',
  },
].map(makeApp);
apps.push(awsConsoleApp, awsIamIcAccountApp);

export const gcpCloudApp = makeApp({
  name: 'cloud-app',
  description: 'Cloud app (GCP)',
  uri: 'cloud://GCP',
  publicAddr: 'cloud-app.teleport.example.com',
  fqdn: 'cloud-app.teleport.example.com',
});

export const moreApps = [
  {
    name: 'Awaggi',
    uri: 'https://jenkins.teleport-proxy.com',
    publicAddr: 'jenkins.teleport-proxy.com',
    description: 'This is a Jenkins app',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'jenkins.one',
  },
  {
    name: 'Vugetje',
    uri: 'https://jenkins.teleport-proxy.com',
    publicAddr: 'jenkins.teleport-proxy.com',
    description: 'This is a Jenkins app',
    awsConsole: false,
    labels: [{ name: 'icon', value: 'jenkins' }],
    clusterId: 'one',
    fqdn: 'jenkins.two',
  },
  {
    name: 'Gaphamoc',
    uri: 'https://grafana.teleport-proxy.com',
    publicAddr: 'grafana.teleport-proxy.com',
    description: 'This is a Grafana app',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
    fqdn: 'g.one',
  },
  {
    kind: 'app',
    name: 'Nidmodug',
    description: 'Nidmodug',
    uri: 'https://dev-1.okta.com/home/dev-1',
    publicAddr: '234.dev-test.teleport',
    fqdn: '234.dev-test.teleport',
    clusterId: 'dev-test.teleport',
    labels: [
      {
        name: 'okta/org',
        value: 'https://dev-test.okta.com',
      },
      {
        name: 'teleport.dev/origin',
        value: 'okta',
      },
    ],
    awsConsole: false,
    friendlyName: 'Teleport Okta',
  },
  {
    name: 'Jechedlak',
    uri: 'https://slack.teleport-proxy.com',
    publicAddr: 'slack.teleport-proxy.com',
    description: 'This is the employee slack channel',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'prod' },
      { name: 'icon', value: 'slack' },
    ],
    clusterId: 'one',
    fqdn: 's.one',
  },
  {
    name: 'Kejufaz',
    uri: '',
    publicAddr: '',
    description: 'SAML Application',
    awsConsole: false,
    labels: [],
    clusterId: 'one',
    fqdn: '',
    samlApp: true,
    samlAppSSOUrl: '',
  },
  {
    name: 'Vimasim',
    uri: '',
    publicAddr: '',
    description: 'SAML Application',
    awsConsole: false,
    labels: [],
    clusterId: 'one',
    fqdn: '',
    samlApp: true,
    friendlyName: 'Okta Friendly',
    samlAppSSOUrl: '',
  },
  {
    name: 'Wugasen',
    uri: 'https://mattermost1.teleport-proxy.com',
    publicAddr: 'mattermost.teleport-proxy.com',
    description: 'This is a Mattermost app',
    awsConsole: false,
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'two' },
    ],
    clusterId: 'one',
    fqdn: 'mattermost.one',
  },
  {
    name: 'Sesenno',
    uri: 'tcp://some-address',
    publicAddr: '',
    description: 'This is a TCP app',
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
  },
  {
    name: 'Uzeamfok',
    uri: 'https://console.aws.amazon.com/ec2/v2/home',
    publicAddr: 'awsconsole-1.teleport-proxy.com',
    labels: [
      { name: 'aws_account_id', value: 'A1234' },
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'two' },
    ],
    description: 'This is an AWS Console app',
    awsConsole: true,
    awsRoles: [
      {
        name: 'role name',
        arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
        display: 'EC2FullAccess',
      },
      {
        name: 'other role name',
        arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
        display: 'ReallyLonReallyLonggggggEC2FullAccess',
      },
      {
        name: 'thisthing',
        arn: 'arn:aws:iam::joe123:role/EC2ReadOnly',
        display: 'EC2ReadOnly',
      },
      ...new Array(20).fill(undefined).map((_, index) => {
        return {
          name: `long-${index}`,
          arc: `arn:aws:iam::${index}`,
          display: `LONG${index}`,
        };
      }),
    ],
    clusterId: 'one',
    fqdn: 'awsconsole-1.com',
  },
  {
    name: 'Ruskilij',
    uri: 'cloud://some-address',
    publicAddr: '',
    description: 'This is a Cloud specific app',
    labels: [
      { name: 'env', value: 'dev' },
      { name: 'cluster', value: 'one' },
    ],
    clusterId: 'one',
  },
].map(makeApp);
moreApps.push(gcpCloudApp);
