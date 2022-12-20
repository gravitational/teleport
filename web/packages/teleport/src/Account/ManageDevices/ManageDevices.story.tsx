/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

import { State } from './useManageDevices';
import { ManageDevices } from './ManageDevices';

export default {
  title: 'Teleport/Account/Manage Devices',
};

export const Loaded = () => <ManageDevices {...props} />;

export const LoadedMfaOff = () => (
  <ManageDevices {...props} mfaDisabled={true} />
);

export const Processing = () => (
  <ManageDevices
    {...props}
    fetchDevicesAttempt={{
      status: 'processing',
    }}
  />
);

export const Failed = () => (
  <ManageDevices
    {...props}
    fetchDevicesAttempt={{
      status: 'failed',
      statusText: 'failed to fetch devices',
    }}
  />
);

export const RemoveDialog = () => (
  <ManageDevices
    {...props}
    isRemoveDeviceVisible={true}
    token="123"
    deviceToRemove={{ id: '1', name: 'iphone 12' }}
  />
);

export const RemoveDialogFailed = () => (
  <ManageDevices
    {...props}
    isRemoveDeviceVisible={true}
    token="123"
    deviceToRemove={{ id: '1', name: 'iphone 12' }}
    removeDevice={() => Promise.reject(new Error('server error'))}
  />
);

export const RestrictedTokenCreateProcessing = () => (
  <ManageDevices
    {...props}
    createRestrictedTokenAttempt={{
      status: 'processing',
    }}
  />
);

export const RestrictedTokenCreateFailed = () => (
  <ManageDevices
    {...props}
    createRestrictedTokenAttempt={{
      status: 'failed',
      statusText: 'failed to create privilege token',
    }}
  />
);

const props: State = {
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
  devices: [
    {
      id: '1',
      description: 'Authenticator App',
      name: 'iphone 12',
      registeredDate: new Date(1628799417000),
      lastUsedDate: new Date(1628799417000),
    },
    {
      id: '2',
      description: 'Hardware Key',
      name: 'solokey',
      registeredDate: new Date(1623722252000),
      lastUsedDate: new Date(1623981452000),
    },
    {
      id: '3',
      description: 'Hardware Key',
      name: 'backup yubikey',
      registeredDate: new Date(1618711052000),
      lastUsedDate: new Date(1626472652000),
    },
    {
      id: '4',
      description: 'Hardware Key',
      name: 'yubikey',
      registeredDate: new Date(1612493852000),
      lastUsedDate: new Date(1614481052000),
    },
  ],
};
