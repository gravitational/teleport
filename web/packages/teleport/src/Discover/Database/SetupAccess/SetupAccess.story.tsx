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

import { noAccess, getAcl } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';
import { TeleportProvider } from 'teleport/Discover/Fixtures/fixtures';
import {
  ComponentWrapper,
  getDbMeta,
  getDbResourceSpec,
} from 'teleport/Discover/Fixtures/databases';

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';

import SetupAccess from './SetupAccess';

export default {
  title: 'Teleport/Discover/Database/SetupAccess',
  loaders: [mswLoader],
  parameters: {
    msw: {
      handlers: {
        fetchUser: rest.get(cfg.api.userWithUsernamePath, (req, res, ctx) =>
          res(
            ctx.json({
              name: 'llama',
              roles: ['access'],
              traits: dynamicTraits,
            })
          )
        ),
      },
    },
  },
};

initialize();

export const NoTraits = () => {
  const meta = getDbMeta();
  meta.db.users = [];
  meta.db.names = [];
  return (
    <TeleportProvider
      agentMeta={meta}
      resourceKind={ResourceKind.Database}
      resourceSpec={getDbResourceSpec(
        DatabaseEngine.Postgres,
        DatabaseLocation.Aws
      )}
    >
      <SetupAccess />
    </TeleportProvider>
  );
};
NoTraits.parameters = {
  msw: {
    handlers: {
      fetchUser: [
        rest.get(cfg.api.userWithUsernamePath, (req, res, ctx) =>
          res(ctx.json({}))
        ),
      ],
    },
  },
};

export const WithTraitsAwsPostgres = () => (
  <TeleportProvider
    resourceSpec={getDbResourceSpec(
      DatabaseEngine.Postgres,
      DatabaseLocation.Aws
    )}
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsAwsPostgresAutoEnroll = () => {
  const meta = getDbMeta();
  meta.db = undefined;
  return (
    <TeleportProvider
      resourceKind={ResourceKind.Database}
      resourceSpec={getDbResourceSpec(
        DatabaseEngine.Postgres,
        DatabaseLocation.Aws
      )}
      agentMeta={
        {
          ...meta,
          autoDiscovery: {
            config: {
              name: 'some-name',
              discoveryGroup: 'some-group',
              aws: [
                {
                  types: ['rds'],
                  regions: ['us-east-1'],
                  tags: {},
                  integration: 'some-integration',
                },
              ],
            },
          },
        } as any
      }
    >
      <SetupAccess />
    </TeleportProvider>
  );
};

export const WithTraitsAwsMySql = () => (
  <ComponentWrapper>
    <SetupAccess />
  </ComponentWrapper>
);

export const WithTraitsPostgres = () => (
  <ComponentWrapper>
    <SetupAccess />
  </ComponentWrapper>
);

export const WithTraitsMongo = () => (
  <TeleportProvider
    resourceSpec={getDbResourceSpec(DatabaseEngine.MongoDb)}
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsMySql = () => (
  <TeleportProvider
    resourceSpec={getDbResourceSpec(DatabaseEngine.MySql)}
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const NoAccess = () => (
  <TeleportProvider
    customAcl={{ ...getAcl(), users: noAccess }}
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
    resourceSpec={getDbResourceSpec(
      DatabaseEngine.Postgres,
      DatabaseLocation.Aws
    )}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const SsoUser = () => (
  <TeleportProvider
    authType="sso"
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
    resourceSpec={getDbResourceSpec(
      DatabaseEngine.Postgres,
      DatabaseLocation.Aws
    )}
  >
    <SetupAccess />
  </TeleportProvider>
);

const dynamicTraits = {
  databaseNames: ['dynamicName1', 'dynamicName2'],
  databaseUsers: ['dynamicUser1', 'dynamicUser2'],
  logins: [],
  kubeUsers: [],
  kubeGroups: [],
  windowsLogins: [],
  awsRoleArns: [],
};
