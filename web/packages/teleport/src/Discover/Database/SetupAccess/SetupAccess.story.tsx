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
