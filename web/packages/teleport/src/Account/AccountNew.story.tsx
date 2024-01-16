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

import React from 'react';

import { Account, AccountProps } from './AccountNew';

export default {
  title: 'Teleport/Account (new)',
  component: Account,
};

export const Loaded = () => <Account {...props} />;

export const LoadedMfaOff = () => <Account {...props} mfaDisabled />;

export const LoadingDevices = () => (
  <Account
    {...props}
    devices={[]}
    fetchDevicesAttempt={{ status: 'processing' }}
  />
);

export const LoadingDevicesFailed = () => (
  <Account
    {...props}
    devices={[]}
    fetchDevicesAttempt={{
      status: 'failed',
      statusText: 'failed to fetch devices',
    }}
  />
);

export const RemoveDialog = () => (
  <Account
    {...props}
    isRemoveDeviceVisible={true}
    token="123"
    deviceToRemove={{ id: '1', name: 'iphone 12' }}
  />
);

export const RemoveDialogFailed = () => (
  <Account
    {...props}
    isRemoveDeviceVisible={true}
    token="123"
    deviceToRemove={{ id: '1', name: 'iphone 12' }}
    removeDevice={() => Promise.reject(new Error('server error'))}
  />
);

export const RestrictedTokenCreateProcessing = () => (
  <Account
    {...props}
    createRestrictedTokenAttempt={{
      status: 'processing',
    }}
  />
);

export const RestrictedTokenCreateFailed = () => (
  <Account
    {...props}
    createRestrictedTokenAttempt={{
      status: 'failed',
      statusText: 'failed to create privilege token',
    }}
  />
);

const props: AccountProps = {
  token: '',
  setToken: () => null,
  onAddDevice: () => null,
  hideAddDevice: () => null,
  fetchDevices: () => null,
  fetchDevicesAttempt: { status: 'success' },
  createRestrictedTokenAttempt: { status: '' },
  deviceToRemove: null,
  onRemoveDevice: () => null,
  removeDevice: () => null,
  mfaDisabled: false,
  hideReAuthenticate: () => null,
  hideRemoveDevice: () => null,
  isReAuthenticateVisible: false,
  isAddDeviceVisible: false,
  isRemoveDeviceVisible: false,
  isSso: false,
  restrictNewDeviceUsage: null,
  devices: [
    {
      id: '1',
      description: 'Hardware Key',
      name: 'touch_id',
      registeredDate: new Date(1628799417000),
      lastUsedDate: new Date(1628799417000),
      residentKey: true,
    },
    {
      id: '2',
      description: 'Hardware Key',
      name: 'solokey',
      registeredDate: new Date(1623722252000),
      lastUsedDate: new Date(1623981452000),
      residentKey: true,
    },
    {
      id: '3',
      description: 'Hardware Key',
      name: 'backup yubikey',
      registeredDate: new Date(1618711052000),
      lastUsedDate: new Date(1626472652000),
      residentKey: true,
    },
    {
      id: '4',
      description: 'Hardware Key',
      name: 'yubikey',
      registeredDate: new Date(1612493852000),
      lastUsedDate: new Date(1614481052000),
      residentKey: true,
    },
    {
      id: '5',
      description: 'Hardware Key',
      name: 'yubikey-mfa',
      registeredDate: new Date(1612493852000),
      lastUsedDate: new Date(1614481052000),
      residentKey: false,
    },
    {
      id: '6',
      description: 'Authenticator App',
      name: 'iphone 12',
      registeredDate: new Date(1628799417000),
      lastUsedDate: new Date(1628799417000),
      residentKey: false,
    },
  ],
};
