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

import { DatabaseEngine } from '../../SelectResource';
import { MutualTlsView } from './MutualTls';
import type { State } from './useMutualTls';

export default {
  title: 'Teleport/Discover/Database/MutualTls',
};

export const LoadedPostgres = () => (
  <MemoryRouter>
    <DiscoverBox>
      <MutualTlsView {...props} />
    </DiscoverBox>
  </MemoryRouter>
);

export const LoadedMongo = () => (
  <MemoryRouter>
    <DiscoverBox>
      <MutualTlsView {...props} dbEngine={DatabaseEngine.MongoDb} />
    </DiscoverBox>
  </MemoryRouter>
);

export const LoadedSqlMaria = () => (
  <MemoryRouter>
    <DiscoverBox>
      <MutualTlsView {...props} dbEngine={DatabaseEngine.MySql} />
    </DiscoverBox>
  </MemoryRouter>
);

export const NoPerm = () => (
  <MemoryRouter>
    <DiscoverBox>
      <MutualTlsView {...props} canUpdateDatabase={false} />
    </DiscoverBox>
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <DiscoverBox>
      <MutualTlsView
        {...props}
        attempt={{ status: 'failed', statusText: 'some error message' }}
      />
    </DiscoverBox>
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <DiscoverBox>
      <MutualTlsView {...props} attempt={{ status: 'processing' }} />
    </DiscoverBox>
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
