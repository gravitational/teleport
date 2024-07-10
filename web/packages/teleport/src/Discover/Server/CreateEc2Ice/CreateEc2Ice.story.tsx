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

import { http, HttpResponse, delay } from 'msw';

import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';
import {
  Ec2InstanceConnectEndpoint,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { CreateEc2Ice } from './CreateEc2Ice';

export default {
  title: 'Teleport/Discover/Server/EC2/CreateEICE',
};

const mockedCreatedEc2Ice: Ec2InstanceConnectEndpoint = {
  name: 'test-eice',
  state: 'create-complete',
  stateMessage: '',
  dashboardLink: 'goteleport.com',
  subnetId: 'test-subnetid',
  vpcId: 'test',
};

const deployEndpointSuccess = http.post(
  cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'),
  () => HttpResponse.json({ name: 'test-eice' })
);

let tick = 0;
const ec2IceEndpointWithTick = http.post(
  cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'),
  () => {
    if (tick == 1) {
      tick = 0; // reset, the polling will be finished by this point.
      return HttpResponse.json({
        ec2Ices: [mockedCreatedEc2Ice],
      });
    }
    tick += 1;
    return HttpResponse.json({
      ec2Ices: [{ ...mockedCreatedEc2Ice, state: 'create-in-progress' }],
    });
  }
);

export const AutoDiscoverEnabled = () => (
  <>
    <Info>
      Devs: after clicking next, wait 10 seconds for in progress to change to
      created
    </Info>
    <Component autoDiscover={true} />
  </>
);
AutoDiscoverEnabled.parameters = {
  msw: {
    handlers: [deployEndpointSuccess, ec2IceEndpointWithTick],
  },
};

export const ListSecurityGroupsLoading = () => <Component />;

ListSecurityGroupsLoading.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getListSecurityGroupsUrl('test-oidc'), () =>
        delay('infinite')
      ),
    ],
  },
};

export const ListSecurityGroupsFail = () => <Component />;

ListSecurityGroupsFail.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getListSecurityGroupsUrl('test-oidc'), () =>
        HttpResponse.json(
          {
            message: 'some error when trying to list security groups',
          },
          { status: 403 }
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
      http.post(cfg.getListSecurityGroupsUrl('test-oidc'), () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'), () =>
        HttpResponse.json(
          {
            message: 'some error when trying to initiate the deployment',
          },
          { status: 403 }
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
      http.post(cfg.getListSecurityGroupsUrl('test-oidc'), () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'), () =>
        HttpResponse.json({
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
      ),
      deployEndpointSuccess,
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
      http.post(cfg.getListSecurityGroupsUrl('test-oidc'), () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'), () =>
        HttpResponse.json({
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
      ),
      deployEndpointSuccess,
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
      http.post(cfg.getListSecurityGroupsUrl('test-oidc'), () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.getDeployEc2InstanceConnectEndpointUrl('test-oidc'), () =>
        HttpResponse.json({ name: 'test-eice' })
      ),
      http.post(cfg.getListEc2InstanceConnectEndpointsUrl('test-oidc'), () =>
        HttpResponse.json({
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
      ),
      http.post(cfg.getClusterNodesUrlNoParams('localhost'), async () => {
        await delay(2000);
        return HttpResponse.json({
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
        });
      }),
    ],
  },
};

const Component = ({ autoDiscover = false }: { autoDiscover?: boolean }) => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      awsRegion: 'us-east-1',
      resourceName: 'node-name',
      agentMatcherLabels: [],
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
      autoDiscovery: autoDiscover
        ? {
            config: { name: '', discoveryGroup: '', aws: [] },
            requiredVpcsAndSubnets: {
              'vpc-1': ['subnet-1'],
              'vpc-2': ['subnet-2'],
            },
          }
        : undefined,
    },
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
