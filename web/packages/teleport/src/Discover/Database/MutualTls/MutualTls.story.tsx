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

import { DatabaseEngine } from '../../SelectResource';

import { MutualTlsView } from './MutualTls';

import type { State } from './useMutualTls';

export default {
  title: 'Teleport/Discover/Database/MutualTls',
};

export const LoadedPostgres = () => (
  <MemoryRouter>
    <MutualTlsView {...props} />
  </MemoryRouter>
);

export const LoadedMongo = () => (
  <MemoryRouter>
    <MutualTlsView {...props} dbEngine={DatabaseEngine.MongoDb} />
  </MemoryRouter>
);

export const LoadedSqlMaria = () => (
  <MemoryRouter>
    <MutualTlsView {...props} dbEngine={DatabaseEngine.MySql} />
  </MemoryRouter>
);

export const NoPerm = () => (
  <MemoryRouter>
    <MutualTlsView {...props} canUpdateDatabase={false} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <MutualTlsView
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <MutualTlsView {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  onNextStep: () => null,
  canUpdateDatabase: true,
  curlCmd: `some curl command`,
  dbEngine: DatabaseEngine.Postgres,
};
