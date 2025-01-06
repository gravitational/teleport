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

import { http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import {
  getDbMeta,
  getDbResourceSpec,
} from 'teleport/Discover/Fixtures/databases';
import { TeleportProvider } from 'teleport/Discover/Fixtures/fixtures';
import { ResourceKind } from 'teleport/Discover/Shared';
import { getAcl, noAccess } from 'teleport/mocks/contexts';

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';
import SetupAccess from './SetupAccess';

export default {
  title: 'Teleport/Discover/Database/SetupAccess',
  parameters: {
    msw: {
      handlers: {
        fetchUser: http.get(cfg.api.userWithUsernamePath, () =>
          HttpResponse.json({
            name: 'llama',
            roles: ['access'],
            traits: dynamicTraits,
          })
        ),
      },
    },
  },
};

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
        http.get(cfg.api.userWithUsernamePath, () => HttpResponse.json({})),
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
  <TeleportProvider
    resourceSpec={getDbResourceSpec(DatabaseEngine.MySql, DatabaseLocation.Aws)}
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
  >
    <SetupAccess />
  </TeleportProvider>
);

export const WithTraitsPostgres = () => (
  <TeleportProvider
    resourceSpec={getDbResourceSpec(DatabaseEngine.Postgres)}
    agentMeta={getDbMeta()}
    resourceKind={ResourceKind.Database}
  >
    <SetupAccess />
  </TeleportProvider>
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
