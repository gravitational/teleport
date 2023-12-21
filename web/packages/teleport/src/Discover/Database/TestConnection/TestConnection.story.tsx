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

import { DatabaseEngine } from '../../SelectResource';

import { TestConnectionView } from './TestConnection';

import type { State } from './useTestConnection';

export default {
  title: 'Teleport/Discover/Database/TestConnection',
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
    kind: 'db',
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
