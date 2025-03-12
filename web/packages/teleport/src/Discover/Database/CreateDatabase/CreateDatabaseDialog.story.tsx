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

import { Info } from 'design/Alert';

import { dbWithoutDbServerExistsErrorMsg, timeoutErrorMsg } from './const';
import {
  CreateDatabaseDialog,
  CreateDatabaseDialogProps,
} from './CreateDatabaseDialog';

export default {
  title: 'Teleport/Discover/Database/CreateDatabase/Dialog',
};

export const Processing = () => <CreateDatabaseDialog {...props} />;

export const Failed = () => (
  <CreateDatabaseDialog
    {...props}
    attempt={{ status: 'failed', statusText: 'some failed text' }}
  />
);

export const Success = () => (
  <CreateDatabaseDialog {...props} attempt={{ status: 'success' }} />
);

export const AllowSkipOnTimeout = () => (
  <>
    <Info>Devs: it should be same state as success</Info>
    <CreateDatabaseDialog
      {...props}
      attempt={{ status: 'failed', statusText: timeoutErrorMsg }}
    />
  </>
);

export const AllowOverwrite = () => (
  <CreateDatabaseDialog
    {...props}
    attempt={{
      status: 'failed',
      statusText: `A database with the name "some-name" ${dbWithoutDbServerExistsErrorMsg}. \
          You can overwrite it, or use a different name and retry.`,
    }}
  />
);

const props: CreateDatabaseDialogProps = {
  pollTimeout: 8080000000,
  attempt: { status: 'processing' },
  retry: () => null,
  close: () => null,
  next: () => null,
  onOverwrite: () => null,
  onTimeout: () => null,
  dbName: 'db-name',
};
