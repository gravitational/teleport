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
  getSelectedAwsPostgresDbMeta,
  resourceSpecAwsRdsMySql,
  resourceSpecAwsRdsPostgres,
  resourceSpecSelfHostedMysql,
  resourceSpecSelfHostedPostgres,
} from 'teleport/Discover/Fixtures/databases';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';
import { getAcl, noAccess } from 'teleport/mocks/contexts';

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
  const meta = getSelectedAwsPostgresDbMeta();
  meta.db.users = [];
  meta.db.names = [];
  return (
    <RequiredDiscoverProviders
      agentMeta={meta}
      resourceSpec={resourceSpecAwsRdsPostgres}
    >
      <SetupAccess />
    </RequiredDiscoverProviders>
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
  <RequiredDiscoverProviders
    resourceSpec={resourceSpecAwsRdsPostgres}
    agentMeta={getSelectedAwsPostgresDbMeta()}
  >
    <SetupAccess />
  </RequiredDiscoverProviders>
);

export const WithTraitsAwsPostgresAutoEnroll = () => {
  const meta = getSelectedAwsPostgresDbMeta();
  meta.db = undefined;
  return (
    <RequiredDiscoverProviders
      resourceSpec={resourceSpecAwsRdsPostgres}
      agentMeta={{
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
        serviceDeploy: {
          method: 'auto',
          selectedSecurityGroups: ['sg-1', 'sg-2'],
          selectedSubnetIds: ['subnet1', 'subnet2'],
        },
      }}
    >
      <SetupAccess />
    </RequiredDiscoverProviders>
  );
};

export const WithTraitsAwsMySql = () => (
  <RequiredDiscoverProviders
    resourceSpec={resourceSpecAwsRdsMySql}
    agentMeta={getSelectedAwsPostgresDbMeta()}
  >
    <SetupAccess />
  </RequiredDiscoverProviders>
);

export const WithTraitsPostgres = () => (
  <RequiredDiscoverProviders
    resourceSpec={resourceSpecSelfHostedPostgres}
    agentMeta={getSelectedAwsPostgresDbMeta()}
  >
    <SetupAccess />
  </RequiredDiscoverProviders>
);

export const WithTraitsMySql = () => (
  <RequiredDiscoverProviders
    resourceSpec={resourceSpecSelfHostedMysql}
    agentMeta={getSelectedAwsPostgresDbMeta()}
  >
    <SetupAccess />
  </RequiredDiscoverProviders>
);

export const NoAccess = () => (
  <RequiredDiscoverProviders
    customAcl={{ ...getAcl(), users: noAccess }}
    agentMeta={getSelectedAwsPostgresDbMeta()}
    resourceSpec={resourceSpecAwsRdsPostgres}
  >
    <SetupAccess />
  </RequiredDiscoverProviders>
);

export const SsoUser = () => (
  <RequiredDiscoverProviders
    authType="sso"
    agentMeta={getSelectedAwsPostgresDbMeta()}
    resourceSpec={resourceSpecAwsRdsPostgres}
  >
    <SetupAccess />
  </RequiredDiscoverProviders>
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
