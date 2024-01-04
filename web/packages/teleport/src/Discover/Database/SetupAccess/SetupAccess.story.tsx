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

import { initSelectedOptionsHelper } from 'teleport/Discover/Shared/SetupAccess';

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';

import { SetupAccess } from './SetupAccess';

import type { State } from 'teleport/Discover/Shared/SetupAccess';

export default {
  title: 'Teleport/Discover/Database/SetupAccess',
};

export const NoTraits = () => (
  <MemoryRouter>
    <SetupAccess {...props} initSelectedOptions={() => []} />
  </MemoryRouter>
);

export const WithTraitsAwsPostgres = () => (
  <MemoryRouter>
    <SetupAccess
      {...props}
      resourceSpec={getDbMeta(DatabaseEngine.Postgres, DatabaseLocation.Aws)}
    />
  </MemoryRouter>
);

export const WithTraitsAwsMySql = () => (
  <MemoryRouter>
    <SetupAccess
      {...props}
      resourceSpec={getDbMeta(DatabaseEngine.MySql, DatabaseLocation.Aws)}
    />
  </MemoryRouter>
);

export const WithTraitsPostgres = () => (
  <MemoryRouter>
    <SetupAccess {...props} />
  </MemoryRouter>
);

export const WithTraitsMongo = () => (
  <MemoryRouter>
    <SetupAccess {...props} resourceSpec={getDbMeta(DatabaseEngine.MongoDb)} />
  </MemoryRouter>
);

export const WithTraitsMySql = () => (
  <MemoryRouter>
    <SetupAccess {...props} resourceSpec={getDbMeta(DatabaseEngine.MySql)} />
  </MemoryRouter>
);

export const NoAccess = () => (
  <MemoryRouter>
    <SetupAccess {...props} canEditUser={false} />
  </MemoryRouter>
);

export const SsoUser = () => (
  <MemoryRouter>
    <SetupAccess {...props} isSsoUser={true} />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  agentMeta: {} as any,
  onProceed: () => null,
  onPrev: () => null,
  fetchUserTraits: () => null,
  isSsoUser: false,
  canEditUser: true,
  getFixedOptions: () => [],
  getSelectableOptions: () => [],
  initSelectedOptions: trait =>
    initSelectedOptionsHelper({ trait, staticTraits, dynamicTraits }),
  dynamicTraits: {} as any,
  staticTraits: {} as any,
  resourceSpec: getDbMeta(DatabaseEngine.Postgres, DatabaseLocation.SelfHosted),
};

const staticTraits = {
  databaseUsers: ['staticUser1', 'staticUser2'],
  databaseNames: ['staticName1', 'staticName2'],
  logins: [],
  kubeUsers: [],
  kubeGroups: [],
  windowsLogins: [],
  awsRoleArns: [],
};

const dynamicTraits = {
  databaseUsers: ['dynamicUser1', 'dynamicUser2'],
  databaseNames: ['dynamicName1', 'dynamicName2'],
  logins: [],
  kubeUsers: [],
  kubeGroups: [],
  windowsLogins: [],
  awsRoleArns: [],
};

function getDbMeta(dbEngine: DatabaseEngine, dbLocation?: DatabaseLocation) {
  return {
    // Only these fields are relevant.
    dbMeta: {
      dbEngine,
      dbLocation,
    },
  } as any;
}
