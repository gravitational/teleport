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

import { PasswordState } from 'teleport/services/user';

import { Account, AccountProps } from './Account';

export default {
  title: 'Teleport/Account',
  component: Account,
};

export const Loaded = () => <Account {...props} />;

export const LoadedNoDevices = () => <Account {...props} devices={[]} />;

export const LoadedPasswordStateUnspecified = () => (
  <Account
    {...props}
    passwordState={PasswordState.PASSWORD_STATE_UNSPECIFIED}
  />
);

export const LoadedPasswordUnset = () => (
  <Account
    {...props}
    devices={props.devices.filter(d => d.usage === 'passwordless')}
    passwordState={PasswordState.PASSWORD_STATE_UNSET}
  />
);

export const LoadedPasskeysOff = () => (
  <Account {...props} canAddPasskeys={false} />
);

export const LoadedMfaOff = () => <Account {...props} canAddMfa={false} />;

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
  fetchDevicesAttempt: { status: 'success' },
  createRestrictedTokenAttempt: { status: '' },
  deviceToRemove: null,
  onRemoveDevice: () => null,
  removeDevice: () => null,
  mfaDisabled: false,
  hideReAuthenticate: () => null,
  hideRemoveDevice: () => null,
  isReAuthenticateVisible: false,
  isRemoveDeviceVisible: false,
  isSso: false,
  newDeviceUsage: null,
  canAddPasskeys: true,
  canAddMfa: true,
  devices: [
    {
      id: '1',
      description: 'Hardware Key',
      name: 'touch_id',
      registeredDate: new Date(1628799417000),
      lastUsedDate: new Date(1628799417000),
      type: 'webauthn',
      usage: 'passwordless',
    },
    {
      id: '2',
      description: 'Hardware Key',
      name: 'solokey',
      registeredDate: new Date(1623722252000),
      lastUsedDate: new Date(1623981452000),
      type: 'webauthn',
      usage: 'passwordless',
    },
    {
      id: '3',
      description: 'Hardware Key',
      name: 'backup yubikey',
      registeredDate: new Date(1618711052000),
      lastUsedDate: new Date(1626472652000),
      type: 'webauthn',
      usage: 'passwordless',
    },
    {
      id: '4',
      description: 'Hardware Key',
      name: 'yubikey',
      registeredDate: new Date(1612493852000),
      lastUsedDate: new Date(1614481052000),
      type: 'webauthn',
      usage: 'passwordless',
    },
    {
      id: '5',
      description: 'Hardware Key',
      name: 'yubikey-mfa',
      registeredDate: new Date(1612493852000),
      lastUsedDate: new Date(1614481052000),
      type: 'webauthn',
      usage: 'mfa',
    },
    {
      id: '6',
      description: 'Authenticator App',
      name: 'iphone 12',
      registeredDate: new Date(1628799417000),
      lastUsedDate: new Date(1628799417000),
      type: 'totp',
      usage: 'mfa',
    },
  ],
  onDeviceAdded: () => {},
  isReauthenticationRequired: false,
  addDeviceWizardVisible: false,
  closeAddDeviceWizard: () => {},
  passwordState: PasswordState.PASSWORD_STATE_SET,
  onPasswordChange: () => {},
};
