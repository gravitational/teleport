/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import makeApp from 'teleport/services/apps/makeApps';

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
        arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
        display: 'EC2FullAccess',
      },
      {
        arn: 'arn:aws:iam::joe123:role/EC2ReadOnly',
        display: 'EC2ReadOnly',
      },
    ],
    clusterId: 'one',
    fqdn: 'awsconsole-1.com',
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
].map(makeApp);
