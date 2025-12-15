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

import { DiscoverBox } from 'teleport/Discover/Shared';

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';
import { CreateDatabaseView } from './CreateDatabase';
import type { State } from './useCreateDatabase';

export default {
  title: 'Teleport/Discover/Database/CreateDatabase',
};

export const InitSelfHostedPostgres = () => (
  <MemoryRouter>
    <DiscoverBox>
      <CreateDatabaseView {...props} />
    </DiscoverBox>
  </MemoryRouter>
);

export const InitSelfHostedMySql = () => (
  <MemoryRouter>
    <DiscoverBox>
      <CreateDatabaseView {...props} dbEngine={DatabaseEngine.MySql} />
    </DiscoverBox>
  </MemoryRouter>
);

export const NoPerm = () => (
  <MemoryRouter>
    <DiscoverBox>
      <CreateDatabaseView {...props} canCreateDatabase={false} />
    </DiscoverBox>
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <DiscoverBox>
      <CreateDatabaseView {...props} attempt={{ status: 'processing' }} />
    </DiscoverBox>
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <DiscoverBox>
      <CreateDatabaseView
        {...props}
        attempt={{
          status: 'failed',
          statusText:
            'invalid database "sfd" address "sfdsdf": address sfdsdf: missing port in address',
        }}
      />
    </DiscoverBox>
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
