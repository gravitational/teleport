/**
 * Copyright 2023 Gravitational, Inc.
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

import React, { useEffect } from 'react';
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
} from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';

import { EnrollRdsDatabase } from './EnrollRdsDatabase';

initialize();
const defaultIsCloud = cfg.isCloud;

export default {
  title: 'Teleport/Discover/Database/EnrollRds',
  loaders: [mswLoader],
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isCloud = defaultIsCloud;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const InstanceList = () => <Component />;
InstanceList.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.awsRdsDbListPath, (req, res, ctx) =>
        res(ctx.json({ databases: rdsInstances }))
      ),
      rest.get(cfg.api.databasesPath, (req, res, ctx) =>
        res(ctx.json({ items: [rdsInstances[2]] }))
      ),
      rest.post(cfg.api.databasesPath, (req, res, ctx) => res(ctx.json({}))),
      rest.post(cfg.api.discoveryConfigPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.databaseServicesPath, (req, res, ctx) =>
        res(
          ctx.json({ services: [{ name: 'test', matchers: { '*': ['*'] } }] })
        )
      ),
      rest.get(cfg.api.databaseServicesPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.post(cfg.api.awsRdsDbRequiredVpcsPath, (req, res, ctx) =>
        res(ctx.json({ vpcMapOfSubnets: {} }))
      ),
    ],
  },
};

export const InstanceListForCloud = () => {
  cfg.isCloud = true;
  return <Component />;
};
InstanceListForCloud.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.awsRdsDbListPath, (req, res, ctx) =>
        res(ctx.json({ databases: rdsInstances }))
      ),
      rest.get(cfg.api.databasesPath, (req, res, ctx) =>
        res(ctx.json({ items: [rdsInstances[2]] }))
      ),
      rest.post(cfg.api.discoveryConfigPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.get(cfg.api.databaseServicesPath, (req, res, ctx) =>
        res(
          ctx.json({
            items: [
              { name: 'test', resource_matchers: [{ labels: { '*': ['*'] } }] },
            ],
          })
        )
      ),
      rest.get(cfg.api.databaseServicesPath, (req, res, ctx) =>
        res(ctx.json({}))
      ),
      rest.post(cfg.api.awsRdsDbRequiredVpcsPath, (req, res, ctx) =>
        res(ctx.json({ vpcMapOfSubnets: { 'vpc-1': ['subnet1'] } }))
      ),
    ],
  },
};

export const InstanceListLoading = () => <Component />;
InstanceListLoading.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.awsRdsDbListPath, (req, res, ctx) =>
        res(ctx.delay('infinite'))
      ),
    ],
  },
};

export const WithAwsPermissionsError = () => <Component />;
WithAwsPermissionsError.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.awsRdsDbListPath, (req, res, ctx) =>
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
      rest.post(cfg.api.awsRdsDbListPath, (req, res, ctx) =>
        res(ctx.status(404))
      ),
    ],
  },
};

const Component = () => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'db-name',
      agentMatcherLabels: [],
      db: {} as any,
      selectedAwsRdsDb: {} as any,
      node: {} as any,
      awsIntegration: {
        kind: IntegrationKind.AwsOidc,
        name: 'test-oidc',
        resourceType: 'integration',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
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
    resourceSpec: {
      dbMeta: {
        location: DatabaseLocation.Aws,
        engine: DatabaseEngine.Postgres,
      },
    } as any,
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
        { pathname: cfg.routes.discover, state: { entity: 'database' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>
          <Info>Devs: Select any region to see story state</Info>
          <EnrollRdsDatabase />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

const rdsInstances = [
  {
    protocol: 'postgres',
    name: 'rds-1',
    uri: 'rds-1-uri',
    labels: [{ name: 'os', value: 'mac' }],
    aws: {
      status: 'available',
      account_id: '123456789012',
      region: 'us-east-1',
      rds: {
        subnets: ['subnet-1', 'subnet-2'],
        vpc_id: 'vpc-id-1',
        resource_id: 'rds-1-resource-id',
      },
    },
  },
  {
    protocol: 'postgres',
    name: 'rds-2',
    uri: 'rds-2-uri',
    labels: [{ name: 'os', value: 'mac' }],
    aws: {
      status: 'failed',
      account_id: '123456789012',
      region: 'us-east-1',
      rds: {
        subnets: ['subnet-1', 'subnet-2'],
        vpc_id: 'vpc-id-1',
        resource_id: 'rds-2-resource-id',
      },
    },
  },
  {
    protocol: 'postgres',
    name: 'rds-3',
    uri: 'rds-3-uri',
    labels: [{ name: 'os', value: 'mac' }],
    aws: {
      status: 'available',
      account_id: '123456789012',
      region: 'us-east-1',
      rds: {
        subnets: ['subnet-1', 'subnet-2'],
        vpc_id: 'vpc-id-1',
        resource_id: 'rds-3-resource-id',
      },
    },
  },
  {
    protocol: 'postgres',
    name: 'rds-4',
    uri: 'rds-4-uri',
    labels: [{ name: 'os', value: 'mac' }],
    aws: {
      status: 'deleting',
      account_id: '123456789012',
      region: 'us-east-1',
      rds: {
        subnets: ['subnet-1', 'subnet-2'],
        vpc_id: 'vpc-id-1',
        resource_id: 'rds-4-resource-id',
      },
    },
  },
  {
    protocol: 'postgres',
    name: 'rds-5',
    uri: 'rds-5-uri',
    labels: [
      { name: 'os', value: 'windows' },
      { name: 'fruit', value: 'banana' },
    ],
    aws: {
      status: 'available',
      account_id: '123456789012',
      region: 'us-east-1',
      rds: {
        subnets: ['subnet-1', 'subnet-2'],
        vpc_id: 'vpc-id-1',
        resource_id: 'rds-5-resource-id',
      },
    },
  },
];
