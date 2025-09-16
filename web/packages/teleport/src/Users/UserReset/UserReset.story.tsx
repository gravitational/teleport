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

import { UserReset } from './UserReset';

export default {
  title: 'Teleport/Users/UserReset',
};

export const Processing = () => {
  return <UserReset {...props} attempt={{ status: 'processing' }} />;
};

export const Success = () => {
  return <UserReset {...props} attempt={{ status: 'success' }} />;
};

export const Failed = () => {
  return (
    <UserReset
      {...props}
      attempt={{ status: 'failed', statusText: 'some server error' }}
    />
  );
};

const props = {
  username: 'smith',
  token: {
    value: '0c536179038b386728dfee6602ca297f',
    expires: new Date('2021-04-08T07:30:00Z'),
    username: 'Lester',
  },
  onReset() {},
  onClose() {},
  attempt: {
    status: 'processing',
    statusText: '',
  },
};
