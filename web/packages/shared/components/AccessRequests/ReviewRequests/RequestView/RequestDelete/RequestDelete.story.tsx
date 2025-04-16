/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import {
  makeEmptyAttempt,
  makeErrorAttempt,
  makeProcessingAttempt,
} from 'shared/hooks/useAsync';

import { RequestDelete } from './RequestDelete';

export default {
  title: 'Shared/AccessRequests/RequestDelete',
};

export const Loaded = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      deleteRequestAttempt={makeEmptyAttempt()}
    />
  );
};

export const Processing = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      deleteRequestAttempt={makeProcessingAttempt()}
    />
  );
};

export const Failed = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      deleteRequestAttempt={makeErrorAttempt(new Error('server error'))}
    />
  );
};

export const Approved = () => {
  return (
    <RequestDelete
      {...props}
      deleteRequestAttempt={makeEmptyAttempt()}
      requestState="APPROVED"
    />
  );
};

const props = {
  requestId: '5ee98d44-de9d-5103-a7cd-072b1ff76253',
  user: 'admin',
  roles: ['dba'],
  onDelete: () => null,
  onClose: () => null,
};
