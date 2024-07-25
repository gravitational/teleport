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

import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { http, HttpResponse, delay } from 'msw';
import { Info } from 'design/Alert';
import { withoutQuery } from 'web/packages/build/storybook';

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

export const InstanceList = () => <Component />;
InstanceList.parameters = {
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
      http.post(cfg.api.awsRdsDbRequiredVpcsPath, () =>
        HttpResponse.json({ vpcMapOfSubnets: {} })
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
      http.post(cfg.api.awsRdsDbRequiredVpcsPath, () =>
        HttpResponse.json({ vpcMapOfSubnets: { 'vpc-1': ['subnet1'] } })
      ),
    ],
  },
};

export const InstanceListLoading = () => {
  cfg.isCloud = true;
  return <Component />;
};
InstanceListLoading.parameters = {
  msw: {
    handlers: [http.post(cfg.api.awsRdsDbListPath, () => delay('infinite'))],
  },
};

export const WithAwsPermissionsError = () => <Component />;
WithAwsPermissionsError.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json(
          {
            message: 'StatusCode: 403, RequestID: operation error',
          },
          { status: 403 }
        )
      ),
    ],
  },
};

export const WithOtherError = () => <Component />;
WithOtherError.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsRdsDbListPath, () =>
        HttpResponse.json(
          {
            error: { message: 'Whoops, something went wrong.' },
          },
          { status: 404 }
        )
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
