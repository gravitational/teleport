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
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { withoutQuery } from 'web/packages/build/storybook';

import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  DatabaseEngine,
  DatabaseLocation,
} from 'teleport/Discover/SelectResource';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { EnrollRdsDatabase } from './EnrollRdsDatabase';

const defaultIsCloud = cfg.isCloud;
const databasesPathWithoutQuery = withoutQuery(cfg.api.databasesPath);

export default {
  title: 'Teleport/Discover/Database/EnrollRds',
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

export const SelfHostedFlow = () => <Component />;
SelfHostedFlow.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json({ databases: rdsInstances })
      ),
      http.get(databasesPathWithoutQuery, () =>
        HttpResponse.json({ items: [rdsInstances[2]] })
      ),
      http.post(databasesPathWithoutQuery, () => HttpResponse.json({})),
      http.post(cfg.api.discoveryConfigPath, () => HttpResponse.json({})),
      http.get(cfg.api.databaseServicesPath, () =>
        HttpResponse.json({
          services: [{ name: 'test', matchers: { '*': ['*'] } }],
        })
      ),
      http.get(cfg.api.databaseServicesPath, () => HttpResponse.json({})),
      http.post(cfg.api.awsDatabaseVpcsPath, () => HttpResponse.json({ vpcs })),
    ],
  },
};

export const CloudFlow = () => {
  cfg.isCloud = true;
  return <Component />;
};
CloudFlow.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json({ databases: rdsInstances })
      ),
      http.get(databasesPathWithoutQuery, () =>
        HttpResponse.json({ items: [rdsInstances[2]] })
      ),
      http.post(cfg.api.discoveryConfigPath, () => HttpResponse.json({})),
      http.get(cfg.api.databaseServicesPath, () =>
        HttpResponse.json({
          items: [
            { name: 'test', resource_matchers: [{ labels: { '*': ['*'] } }] },
          ],
        })
      ),
      http.get(cfg.api.databaseServicesPath, () => HttpResponse.json({})),
      http.post(cfg.api.awsDatabaseVpcsPath, () => HttpResponse.json({ vpcs })),
    ],
  },
};

export const NoVpcs = () => {
  return <Component />;
};
NoVpcs.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json({ databases: [] })
      ),
      http.post(
        cfg.api.awsDatabaseVpcsPath,
        () => HttpResponse.json({ vpcs: [] }),
        { once: true }
      ),
      http.post(cfg.api.awsDatabaseVpcsPath, () => HttpResponse.json({ vpcs })),
    ],
  },
};

export const VpcError = () => {
  return <Component />;
};
VpcError.parameters = {
  msw: {
    handlers: [
      http.post(
        cfg.api.awsDatabaseVpcsPath,
        () =>
          HttpResponse.json(
            {
              error: { message: 'Whoops, error fetching required vpcs.' },
            },
            { status: 404 }
          ),
        { once: true }
      ),
    ],
  },
};

export const SelectedVpcAlreadyExists = () => {
  return <Component />;
};
SelectedVpcAlreadyExists.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json({ databases: rdsInstances })
      ),
      http.get(databasesPathWithoutQuery, () =>
        HttpResponse.json({ items: [rdsInstances[2]] })
      ),
      http.post(cfg.api.awsDatabaseVpcsPath, () =>
        HttpResponse.json({
          vpcs: [
            {
              id: 'Click me, then toggle ON auto enroll',
              ecsServiceDashboardURL: 'http://some-dashboard-url',
            },
            {
              id: 'vpc-1234',
            },
          ],
        })
      ),
    ],
  },
};

export const LoadingVpcs = () => {
  return <Component />;
};
LoadingVpcs.parameters = {
  msw: {
    handlers: [http.post(cfg.api.awsDatabaseVpcsPath, () => delay('infinite'))],
  },
};

export const LoadingDatabases = () => {
  return <Component />;
};
LoadingDatabases.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () => delay('infinite')),
      http.post(cfg.api.awsDatabaseVpcsPath, () => HttpResponse.json({ vpcs })),
    ],
  },
};

export const WithAwsPermissionsError = () => <Component />;
WithAwsPermissionsError.parameters = {
  msw: {
    handlers: [
      http.post(
        cfg.api.awsDatabaseVpcsPath,
        () =>
          HttpResponse.json(
            {
              message: 'StatusCode: 403, RequestID: operation error',
            },
            { status: 403 }
          ),
        { once: true }
      ),
      http.post(cfg.api.awsDatabaseVpcsPath, () => HttpResponse.json({ vpcs })),
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json({ databases: [] })
      ),
    ],
  },
};

export const WithDbListError = () => <Component />;
WithDbListError.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsDatabaseVpcsPath, () =>
        HttpResponse.json({
          vpcs,
        })
      ),
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json(
          {
            message: 'Whoops, fetching aws databases error',
          },
          { status: 403 }
        )
      ),
    ],
  },
};

export const WithOneOfDbListError = () => <Component />;
WithOneOfDbListError.parameters = {
  msw: {
    handlers: [
      http.get(databasesPathWithoutQuery, () =>
        HttpResponse.json({ items: [rdsInstances[2]] })
      ),
      http.post(databasesPathWithoutQuery, () => HttpResponse.json({})),
      http.post(cfg.api.awsDatabaseVpcsPath, () =>
        HttpResponse.json({
          vpcs,
        })
      ),
      http.post(cfg.api.awsRdsDbListPath, async req => {
        return (await req.request.json())['rdsType'] === 'instance'
          ? HttpResponse.json({ databases: rdsInstances })
          : HttpResponse.json(
              {
                message: 'Whoops, fetching another aws databases error',
              },
              { status: 403 }
            );
      }),
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

const vpcs = [
  {
    name: '',
    id: 'vpc-341c69a6-1bdb-5521-aad1',
  },
  {
    name: '',
    id: 'vpc-92b8d60f-0f0e-5d31-b5b4',
  },
  {
    name: 'aws-controlsomething-VPC',
    id: 'vpc-d36151d6-8f0e-588d-87a7',
  },
  {
    name: 'eksctl-bob-test-1-cluster/VPC',
    id: 'vpc-fe7203d3-e959-57d4-8f87',
  },
  {
    name: 'Default VPC (DO NOT USE)',
    id: 'vpc-57cbdb9c-0f3e-5efb-bd84',
  },
];
