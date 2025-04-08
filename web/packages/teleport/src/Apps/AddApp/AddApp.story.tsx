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

import { AddApp } from './AddApp';

export default {
  title: 'Teleport/Apps/Add',
};

export const Created = () => (
  <AddApp {...props} attempt={{ status: 'success' }} />
);

export const Loaded = () => {
  return <AddApp {...props} />;
};

export const Processing = () => (
  <AddApp {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <AddApp
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

export const ManuallyProcessing = () => (
  <AddApp {...props} automatic={false} attempt={{ status: 'processing' }} />
);

export const ManuallyWithToken = () => <AddApp {...props} automatic={false} />;

export const ManuallyWithoutTokenLocal = () => (
  <AddApp {...props} automatic={false} attempt={{ status: 'failed' }} />
);

export const ManuallyWithoutTokenSSO = () => (
  <AddApp
    {...props}
    automatic={false}
    attempt={{ status: 'failed' }}
    isAuthTypeLocal={false}
  />
);

const props = {
  isEnterprise: false,
  isAuthTypeLocal: true,
  user: 'sam',
  automatic: true,
  setAutomatic: () => null,
  createToken: () => Promise.resolve(true),
  onClose: () => null,
  setCmdParams: () => null,
  createJoinToken: () => Promise.resolve(null),
  version: '5.0.0-dev',
  reset: () => null,
  attempt: {
    status: '',
    statusText: '',
  } as any,
  token: {
    id: 'join-token',
    expiryText: '1 hour',
    expiry: null,
    safeName: '',
    isStatic: false,
    method: 'kubernetes',
    roles: [],
    content: '',
  },
};
