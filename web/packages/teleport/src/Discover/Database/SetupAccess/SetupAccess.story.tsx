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
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import {
  TeleportProvider,
  getDbMeta,
  getDbResourceSpec,
} from 'teleport/Discover/fixtures';
import { noAccess, getAcl } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';

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
              traits: staticTraits,
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
    <TeleportProvider agentMeta={meta}>
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
  >
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsAwsPostgresAutoEnroll = () => {
  const meta = getDbMeta();
  meta.db = undefined;
  return (
    <TeleportProvider
      resourceSpec={getDbResourceSpec(
        DatabaseEngine.Postgres,
        DatabaseLocation.Aws
      )}
      agentMeta={
        {
          ...meta,
          autoDiscoveryConfig: {
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
        } as any
      }
    >
      <SetupAccess />
    </TeleportProvider>
  );
};

export const WithTraitsAwsMySql = () => (
  <TeleportProvider
    agentMeta={getDbMeta()}
    resourceSpec={getDbResourceSpec(DatabaseEngine.MySql, DatabaseLocation.Aws)}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsPostgres = () => (
  <TeleportProvider agentMeta={getDbMeta()}>
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsMongo = () => (
  <TeleportProvider
    resourceSpec={getDbResourceSpec(DatabaseEngine.MongoDb)}
    agentMeta={getDbMeta()}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsMySql = () => (
  <TeleportProvider
    resourceSpec={getDbResourceSpec(DatabaseEngine.MySql)}
    agentMeta={getDbMeta()}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const NoAccess = () => (
  <TeleportProvider
    customAcl={{ ...getAcl(), users: noAccess }}
    agentMeta={getDbMeta()}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const SsoUser = () => (
  <TeleportProvider authType="sso" agentMeta={getDbMeta()}>
    <SetupAccess />
  </TeleportProvider>
);

const staticTraits = {
  databaseUsers: ['staticUser1', 'staticUser2'],
  databaseNames: ['staticName1', 'staticName2'],
  logins: [],
  kubeUsers: [],
  kubeGroups: [],
  windowsLogins: [],
  awsRoleArns: [],
};
