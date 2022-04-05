/*
Copyright 2021-2022 Gravitational, Inc.

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
import { State } from './useAddDevice';
import { AddDevice } from './AddDevice';

export default {
  title: 'Teleport/Account/Manage Devices/Add Device Dialog',
};

export const LoadedWebauthn = () => <AddDevice {...props} />;

export const Failed = () => (
  <AddDevice
    {...props}
    addDeviceAttempt={{ status: 'failed', statusText: 'failed to add device' }}
  />
);

export const QrCodeProcessing = () => (
  <AddDevice
    {...props}
    auth2faType="otp"
    fetchQrCodeAttempt={{ status: 'processing' }}
  />
);

export const QrCodeFailed = () => (
  <AddDevice
    {...props}
    auth2faType="otp"
    fetchQrCodeAttempt={{
      status: 'failed',
      statusText: 'failed to create totp register challenge',
    }}
  />
);

const props: State = {
  addDeviceAttempt: { status: '' },
  fetchQrCodeAttempt: { status: 'success' },
  addTotpDevice: () => null,
  addWebauthnDevice: () => null,
  clearAttempt: () => null,
  onClose: () => null,
  auth2faType: 'on',
  preferredMfaType: 'webauthn',
  qrCode:
    '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',
};
