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

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';

import { CreateDatabaseView } from './CreateDatabase';

import type { State } from './useCreateDatabase';

export default {
  title: 'Teleport/Discover/Database/CreateDatabase',
};

export const InitSelfHostedPostgres = () => (
  <MemoryRouter>
    <CreateDatabaseView {...props} />
  </MemoryRouter>
);

export const InitSelfHostedMySql = () => (
  <MemoryRouter>
    <CreateDatabaseView {...props} dbEngine={DatabaseEngine.MySql} />
  </MemoryRouter>
);

export const NoPerm = () => (
  <MemoryRouter>
    <CreateDatabaseView {...props} canCreateDatabase={false} />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <CreateDatabaseView {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <CreateDatabaseView
      {...props}
      attempt={{
        status: 'failed',
        statusText:
          'invalid database "sfd" address "sfdsdf": address sfdsdf: missing port in address',
      }}
    />
  </MemoryRouter>
);

const props: State = {
  attempt: { status: '' },
  clearAttempt: () => null,
  registerDatabase: () => null,
  fetchDatabaseServers: () => null,
  canCreateDatabase: true,
  pollTimeout: Date.now() + 30000,
  dbEngine: DatabaseEngine.Postgres,
  dbLocation: DatabaseLocation.SelfHosted,
  isDbCreateErr: false,
  prevStep: () => null,
  nextStep: () => null,
  createdDb: {} as any,
};
