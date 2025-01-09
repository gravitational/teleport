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

import { delay, http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
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
import { ResourceKind } from 'teleport/Discover/Shared';

import { AutoDeploy } from './AutoDeploy';

export default {
  title: 'Teleport/Discover/Database/Deploy/Auto',
};

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
      http.post(cfg.api.awsSecurityGroupsListPath, () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.api.awsDeployTeleportServicePath, () =>
        HttpResponse.json({ serviceDashboardUrl: 'some-dashboard-url' })
      ),
      http.post(cfg.api.awsSubnetListPath, () =>
        HttpResponse.json({ subnets: subnetsResponse })
      ),
    ],
  },
};

export const InitWithAutoDiscover = () => {
  const dbMeta = getDbMeta();
  dbMeta.selectedAwsRdsDb = undefined; // there is no selection for discovery
  return (
    <TeleportProvider
      resourceKind={ResourceKind.Database}
      agentMeta={{
        ...dbMeta,
        autoDiscovery: {
          config: { name: '', discoveryGroup: '', aws: [] },
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
InitWithAutoDiscover.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsSecurityGroupsListPath, () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.getAwsRdsDbsDeployServicesUrl('test-integration'), () =>
        HttpResponse.json({
          clusterDashboardUrl: 'some-cluster-dashboard-url',
        })
      ),
      http.post(cfg.api.awsSubnetListPath, () =>
        HttpResponse.json({ subnets: subnetsResponse })
      ),
    ],
  },
};

export const InitWithLabelsWithDeployFailure = () => {
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
InitWithLabelsWithDeployFailure.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsSecurityGroupsListPath, () =>
        HttpResponse.json({ securityGroups: securityGroupsResponse })
      ),
      http.post(cfg.api.awsDeployTeleportServicePath, () =>
        HttpResponse.json(
          {
            error: { message: 'Whoops, something went wrong.' },
          },
          { status: 500 }
        )
      ),
      http.post(cfg.api.awsSubnetListPath, () =>
        HttpResponse.json({ subnets: subnetsResponse })
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
      http.post(cfg.api.awsSecurityGroupsListPath, () =>
        HttpResponse.json(
          {
            message: 'some error when trying to list security groups',
          },
          { status: 403 }
        )
      ),
      http.post(cfg.api.awsSubnetListPath, () =>
        HttpResponse.json(
          {
            error: { message: 'Whoops, error getting subnets' },
          },
          { status: 403 }
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
      http.post(cfg.api.awsSecurityGroupsListPath, () => delay('infinite')),
      http.post(cfg.api.awsSubnetListPath, () => delay('infinite')),
    ],
  },
};

const subnetsResponse = [
  {
    name: 'aws-something-PrivateSubnet1A',
    id: 'subnet-e40cd872-74de-54e3-a081',
    availability_zone: 'us-east-1c',
  },
  {
    name: 'aws-something-PrivateSubnet2A',
    id: 'subnet-e6f9e40e-a7c7-52ab-b8e8',
    availability_zone: 'us-east-1a',
  },
  {
    name: '',
    id: 'subnet-9106bc09-ea32-5216-ae3b',
    availability_zone: 'us-east-1b',
  },
  {
    name: '',
    id: 'subnet-0ee385cf-b090-5cf7-b692',
    availability_zone: 'us-east-1c',
  },
  {
    name: 'something-long-test-1-cluster/SubnetPublicU',
    id: 'subnet-0f0b563e-629f-5921-841d',
    availability_zone: 'us-east-1c',
  },
  {
    name: 'something-long-test-1-cluster/SubnetPrivateUS',
    id: 'subnet-30c9e2f6-65ce-5422-bbc0',
    availability_zone: 'us-east-1c',
  },
];

const securityGroupsResponse = [
  {
    name: 'security-group-1',
    id: 'sg-1',
    description: 'this is security group 1',
    inboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '65535',
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
        groups: [
          { groupId: 'sg-1', description: 'Trusts itself in port range' },
        ],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '8080',
        toPort: '8080',
        groups: [{ groupId: 'sg-3', description: 'Trusts other group' }],
      },
    ],
    outboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '65535',
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
        fromPort: '8080',
        toPort: '8080',
        groups: [
          {
            groupId: 'sg-4',
            description:
              'a trusted group on port 8080 for some reason and this description rambles a lot so the table better truncate it with ellipses but you should still see the full thing by hovering on it :D',
          },
        ],
      },
      {
        ipProtocol: 'tcp',
        fromPort: '2000',
        toPort: '5000',
        cidrs: [
          { cidr: '10.0.0.0/16', description: 'Subnet Mask 255.255.0.0' },
        ],
        groups: [{ groupId: 'sg-4', description: 'some other group' }],
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
        toPort: '65535',
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
      {
        ipProtocol: 'all',
        fromPort: '0',
        toPort: '0',
        groups: [
          { groupId: 'sg-3', description: 'trusts all traffic from sg-3' },
        ],
      },
    ],
    outboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '65535',
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
        fromPort: '22',
        toPort: '22',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything' }],
      },
    ],
  },
  {
    name: 'security-group-4',
    id: 'sg-4',
    description: 'this is security group 4',
    inboundRules: [
      {
        ipProtocol: 'tcp',
        fromPort: '0',
        toPort: '65535',
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
        fromPort: '22',
        toPort: '22',
        cidrs: [{ cidr: '0.0.0.0/0', description: 'Everything ssh' }],
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
