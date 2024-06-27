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
