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
