/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import cfg from 'teleport/config';

import { ResourceKind } from 'teleport/Discover/Shared';

import {
  ComponentWrapper,
  getDbMeta,
  getDbResourceSpec,
} from 'teleport/Discover/Fixtures/databases';

import { TeleportProvider } from 'teleport/Discover/Fixtures/fixtures';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';

import { AutoDeploy } from './AutoDeploy';

export default {
  title: 'Teleport/Discover/Database/Deploy/Auto',
  loaders: [mswLoader],
};

initialize();

export const Init = () => {
  return (
    <ComponentWrapper>
      <AutoDeploy />
    </ComponentWrapper>
  );
};

Init.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getListSecurityGroupsUrl('test-integration'),
        (req, res, ctx) =>
          res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
    ],
  },
};

export const InitWithAutoEnroll = () => {
  return (
    <TeleportProvider
      resourceKind={ResourceKind.Database}
      agentMeta={{
        ...getDbMeta(),
        autoDiscovery: {
          config: { name: '', discoveryGroup: '', aws: [] },
          requiredVpcsAndSubnets: {},
        },
      }}
      resourceSpec={getDbResourceSpec(
        DatabaseEngine.Postgres,
        DatabaseLocation.Aws
      )}
    >
      <AutoDeploy />
    </TeleportProvider>
  );
};
InitWithAutoEnroll.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getListSecurityGroupsUrl('test-integration'),
        (req, res, ctx) =>
          res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
      rest.post(
        cfg.getAwsRdsDbsDeployServicesUrl('test-integration'),
        (req, res, ctx) =>
          res(
            ctx.json({
              clusterDashboardUrl: 'some-cluster-dashboard-url',
            })
          )
      ),
    ],
  },
};

export const InitWithLabels = () => {
  return (
    <TeleportProvider
      resourceKind={ResourceKind.Database}
      resourceSpec={getDbResourceSpec(
        DatabaseEngine.Postgres,
        DatabaseLocation.Aws
      )}
      agentMeta={{
        ...getDbMeta(),
        agentMatcherLabels: [
          { name: 'env', value: 'staging' },
          { name: 'os', value: 'windows' },
        ],
      }}
    >
      <AutoDeploy />
    </TeleportProvider>
  );
};

InitWithLabels.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getListSecurityGroupsUrl('test-integration'),
        (req, res, ctx) =>
          res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
    ],
  },
};

export const InitSecurityGroupsLoadingFailed = () => {
  return (
    <ComponentWrapper>
      <AutoDeploy />
    </ComponentWrapper>
  );
};

InitSecurityGroupsLoadingFailed.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getListSecurityGroupsUrl('test-integration'),
        (req, res, ctx) =>
          res(
            ctx.status(403),
            ctx.json({
              message: 'some error when trying to list security groups',
            })
          )
      ),
    ],
  },
};

export const InitSecurityGroupsLoading = () => {
  return (
    <ComponentWrapper>
      <AutoDeploy />
    </ComponentWrapper>
  );
};

InitSecurityGroupsLoading.parameters = {
  msw: {
    handlers: [
      rest.post(
        cfg.getListSecurityGroupsUrl('test-integration'),
        (req, res, ctx) => res(ctx.delay('infinite'))
      ),
    ],
  },
};

const securityGroupsResponse = [
  {
    name: 'security-group-1',
    id: 'sg-1',
    description: 'this is security group 1',
    inboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '0',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '443',
        toPort: '443',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '192.168.1.0/24', description: 'Subnet Mask 255.255.255.0' },
        ],
      },
    ],
    outboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '0',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '22',
        toPort: '22',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0' },
        ],
      },
    ],
  },
  {
    name: 'security-group-2',
    id: 'sg-2',
    description: 'this is security group 2',
    inboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '0',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '443',
        toPort: '443',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '192.168.1.0/24', description: 'Subnet Mask 255.255.255.0' },
        ],
      },
    ],
    outboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '0',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '22',
        toPort: '22',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0' },
        ],
      },
    ],
  },
  {
    name: 'security-group-3',
    id: 'sg-3',
    description: 'this is security group 3',
    inboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '0',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '443',
        toPort: '443',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '192.168.1.0/24', description: 'Subnet Mask 255.255.255.0' },
        ],
      },
    ],
    outboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '0',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '22',
        toPort: '22',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0' },
        ],
      },
    ],
  },
];
