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
  handleOnTimeout: () => null,
};
