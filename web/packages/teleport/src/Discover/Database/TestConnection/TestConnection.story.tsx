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

import { TestConnectionView } from './TestConnection';

import type { State } from './useTestConnection';

export default {
  title: 'Teleport/Discover/Shared/ConnectionDiagnostic/Database',
};

export const InitMySql = () => (
  <MemoryRouter>
    <TestConnectionView {...props} />
  </MemoryRouter>
);

export const InitPostgres = () => (
  <MemoryRouter>
    <TestConnectionView {...props} dbEngine={DatabaseEngine.Postgres} />
  </MemoryRouter>
);

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  testConnection: () => null,
  nextStep: () => null,
  prevStep: () => null,
  diagnosis: null,
  canTestConnection: true,
  username: 'teleport-username',
  authType: 'local',
  clusterId: 'some-cluster-id',
  db: {
    name: 'dbname',
    description: 'some desc',
    type: 'self-hosted',
    protocol: 'postgres',
    labels: [],
    names: ['name1', 'name2'],
    users: ['user1', 'user2'],
    hostname: 'db-hostname',
  },
  dbEngine: DatabaseEngine.MySql,
  showMfaDialog: false,
  cancelMfaDialog: () => null,
};
