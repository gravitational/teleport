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

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { EnrollEc2Instance } from './EnrollEc2Instance';

export default {
  title: 'Teleport/Discover/Server/EC2/InstanceList',
  loaders: [mswLoader],
};

initialize();

export const InstanceList = () => <Component />;

InstanceList.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListEc2InstancesUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.json({ servers: ec2InstancesResponse }))
      ),
      rest.get(cfg.getClusterNodesUrl('localhost'), (req, res, ctx) =>
        res(ctx.json({ items: [ec2InstancesResponse[2]] }))
      ),
    ],
  },
};

export const InstanceListLoading = () => <Component />;

InstanceListLoading.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListEc2InstancesUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.delay('infinite'))
      ),
    ],
  },
};

export const WithAwsPermissionsError = () => <Component />;

WithAwsPermissionsError.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListEc2InstancesUrl('test-oidc'), (req, res, ctx) =>
        res(
          ctx.status(403),
          ctx.json({ message: 'StatusCode: 403, RequestID: operation error' })
        )
      ),
    ],
  },
};

export const WithOtherError = () => <Component />;

WithOtherError.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.getListEc2InstancesUrl('test-oidc'), (req, res, ctx) =>
        res(ctx.status(404))
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
      node: {} as any,
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
    updateAgentMeta: () => null,
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
          <EnrollEc2Instance />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

const ec2InstancesResponse = [
  {
    id: 'ec2-instance-1',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-1',
    tags: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-1' },
      { name: 'Name', value: 'My EC2 Box 1' },
    ],
    addr: 'ec2.1.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    aws: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-1',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-2',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-2',
    tags: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-2' },
      { name: 'Name', value: 'My EC2 Box 2' },
    ],
    addr: 'ec2.2.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    aws: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-2',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-3',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-3',
    tags: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-3' },
      { name: 'Name', value: 'My EC2 Box 3' },
    ],
    addr: 'ec2.3.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    aws: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-3',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-4',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-4',
    tags: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-4' },
      { name: 'Name', value: 'My EC2 Box 4' },
    ],
    addr: 'ec2.4.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    aws: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-4',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
  {
    id: 'ec2-instance-5',
    kind: 'node',
    clusterId: 'cluster',
    hostname: 'ec2-hostname-5',
    tags: [
      { name: 'teleport.dev/instance-id', value: 'instance-ec2-5' },
      { name: 'Name', value: 'My EC2 Box 5' },
    ],
    addr: 'ec2.5.com',
    tunnel: false,
    subKind: 'openssh-ec2-ice',
    sshLogins: ['test'],
    aws: {
      accountId: 'test-account',
      instanceId: 'instance-ec2-5',
      region: 'us-west-1',
      vpcId: 'test',
      integration: 'test',
      subnetId: 'test',
    },
  },
];
