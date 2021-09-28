import React from 'react';
import { Attempt } from 'shared/hooks/useAttemptNext';
import RemoveDialog from 'teleport/components/MfaDeviceList/RemoveDialog';
import { MfaDevice } from 'teleport/services/mfa';
import { Devices } from './Devices';

export default {
  title: 'TeleportE/Recovery/Flow/Step 3/Devices',
};

export const Loaded = () => <Devices {...props} />;

export const Loading = () => (
  <Devices {...props} attempt={{ status: 'processing' }} />
);

export const Failed = () => (
  <Devices
    {...props}
    attempt={{
      status: 'failed',
      statusText: 'failed to fetch devices',
    }}
  />
);

export const RemoveDeviceDialog = () => (
  <RemoveDialog
    name="yubikey"
    onRemove={() => Promise.reject(new Error('server error'))}
    onCancel={() => null}
  />
);

const props = {
  attempt: { status: 'success' } as Attempt,
  removeDevice: () => null,
  onNext: () => null,
  closeDialog: () => null,
  deviceToRemove: null,
  setDeviceToRemove: () => null,
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
  ] as MfaDevice[],
  mostRecentDevice: {
    id: '1',
    description: 'Authenticator App',
    name: 'iphone 12',
    registeredDate: new Date(1628799417000),
    lastUsedDate: new Date(1628799417000),
  },
};
