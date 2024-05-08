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

import React, { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

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

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/Database/TestConnection',
};

export const InitMySql = () => (
  <MemoryRouter>
    <Provider dbEngine={DatabaseEngine.MySql}>
      <TestConnection />
    </Provider>
  </MemoryRouter>
);

export const InitPostgres = () => (
  <MemoryRouter>
    <Provider dbEngine={DatabaseEngine.Postgres}>
      <TestConnection />
    </Provider>
  </MemoryRouter>
);

const Provider: React.FC<PropsWithChildren<{ dbEngine: DatabaseEngine }>> = ({
  children,
  dbEngine,
}) => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'db-name',
      agentMatcherLabels: [],
      db: {
        kind: 'db',
        name: 'aurora',
        description: 'PostgreSQL 11.6: AWS Aurora ',
        type: 'RDS PostgreSQL',
        protocol: 'postgres',
        labels: [
          { name: 'cluster', value: 'root' },
          { name: 'env', value: 'aws' },
        ],
        hostname: 'aurora-hostname',
        names: ['name1', 'name2', '*'],
        users: ['user1', 'user2', '*'],
      },
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
    onSelectResource: () => null,
    resourceSpec: {
      dbMeta: {
        location: DatabaseLocation.Aws,
        engine: dbEngine,
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
    nextStep: () => null,
    prevStep: () => null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'database' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
