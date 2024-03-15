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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  DiscoverProvider,
  DiscoverContextState,
  NodeMeta,
} from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { CreateEc2Ice } from './CreateEc2Ice';

export default {
  title: 'Teleport/Discover/Server/EC2/CreateEICE',
  loaders: [mswLoader],
};

initialize();

export const ListSecurityGroupsLoading = () => <Component />;

ListSecurityGroupsLoading.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListSecurityGroupsUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.delay('infinite'))
      ),
    ],
  },
};

export const ListSecurityGroupsFail = () => <Component />;

ListSecurityGroupsFail.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListSecurityGroupsUrl('test-oidc'), (req, res, ctx) =>
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

export const DeployEiceFail = () => (
  <>
    <Info width="1000px">To trigger this Story's state, click on "Next."</Info>
    <Component />
  </>
);

DeployEiceFail.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListSecurityGroupsUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
      rest.post(
        cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'),
        (req, res, ctx) =>
          res(
            ctx.status(403),
            ctx.json({
              message: 'some error when trying to initiate the deployment',
            })
          )
      ),
    ],
  },
};

export const CreatingInProgress = () => (
  <>
    <Info width="1000px">To trigger this Story's state, click on "Next."</Info>
    <Component />
  </>
);

CreatingInProgress.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListSecurityGroupsUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
      rest.post(
        cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'),
        (req, res, ctx) =>
          res(
            ctx.json({
              ec2Ices: [
                {
                  name: 'test-eice',
                  state: 'create-in-progress',
                  stateMessage: '',
                  dashboardLink: 'goteleport.com',
                  subnetId: 'test-subnetid',
                },
              ],
              nextToken: '',
            })
          )
      ),
      rest.post(
        cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'),
        (req, res, ctx) => res(ctx.json({ name: 'test-eice' }))
      ),
    ],
  },
};

export const CreatingFailed = () => (
  <>
    {' '}
    <Info width="1000px">
      To trigger this Story's state, click on "Next" and wait 10 seconds.
    </Info>
    <Component />
  </>
);

CreatingFailed.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListSecurityGroupsUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
      rest.post(
        cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'),
        (req, res, ctx) =>
          res(
            ctx.json({
              ec2Ices: [
                {
                  name: 'test-eice',
                  state: 'create-failed',
                  stateMessage: '',
                  dashboardLink: 'goteleport.com',
                  subnetId: 'test-subnetid',
                },
              ],
              nextToken: '',
            })
          )
      ),
      rest.post(
        cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'),
        (req, res, ctx) => res(ctx.json({ name: 'test-eice' }))
      ),
    ],
  },
};

export const CreatingComplete = () => (
  <>
    <Info width="1000px">
      To trigger this Story's state, click on "Next" and wait 10 seconds.
    </Info>
    <Component />
  </>
);

CreatingComplete.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListSecurityGroupsUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.json({ securityGroups: securityGroupsResponse }))
      ),
      rest.post(
        cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'),
        (req, res, ctx) => res(ctx.json({ name: 'test-eice' }))
      ),
      rest.post(
        cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'),
        (req, res, ctx) =>
          res(
            ctx.json({
              ec2Ices: [
                {
                  name: 'test-eice',
                  state: 'create-complete',
                  stateMessage: '',
                  dashboardLink: 'goteleport.com',
                  subnetId: 'test-subnetid',
                },
              ],
              nextToken: '',
            })
          )
      ),
      rest.post(cfg.getClusterNodesUrlNoParams('localhost'), (req, res, ctx) =>
        res(
          ctx.delay(2000), // delay by 2 seconds
          ctx.json({
            id: 'ec2-instance-1',
            kind: 'node',
            clusterId: 'cluster',
            hostname: 'ec2-hostname-1',
            labels: [{ name: 'instance', value: 'ec2-1' }],
            addr: 'ec2.1.com',
            tunnel: false,
            subKind: 'openssh-ec2-ice',
            sshLogins: ['test'],
            aws: {
              accountId: 'test-account',
              instanceId: 'instance-ec2-1',
              region: 'us-east-1',
              vpcId: 'test',
              integration: 'test',
              subnetId: 'test',
            },
          })
        )
      ),
    ],
  },
};

const Component = () => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'node-name',
      agentMatcherLabels: [],
      db: {} as any,
      selectedAwsRdsDb: {} as any,
      node: {
        kind: 'node',
        subKind: 'openssh-ec2-ice',
        id: 'test-node',
        hostname: 'test-node-hostname',
        clusterId: 'localhost',
        labels: [],
        addr: 'test',
        tunnel: false,
        sshLogins: [],
        awsMetadata: {
          accountId: 'test-account',
          integration: 'test-oidc',
          instanceId: 'i-test',
          subnetId: 'test',
          vpcId: 'test-vpc',
          region: 'us-east-1',
        },
      },
      awsIntegration: {
        kind: IntegrationKind.AwsOidc,
        name: 'test-oidc',
        resourceType: 'integration',
        spec: {
          roleArn: 'arn-123',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      },
    } as NodeMeta,
    updateAgentMeta: agentMeta => {
      discoverCtx.agentMeta = agentMeta;
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {} as any,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  cfg.proxyCluster = 'localhost';
  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'server' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>
          <CreateEc2Ice />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
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
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0"' },
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
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0"' },
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
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0"' },
        ],
      },
    ],
  },
];
