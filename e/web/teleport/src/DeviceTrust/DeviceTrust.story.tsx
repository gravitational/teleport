import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'e-teleport/services/devices/types';

import type { State } from './useDevices';

export default {
  title: 'TeleportE/DeviceTrust',
};

export function Empty() {
  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust {...props} items={[]} />
    </ContextProvider>
  );
}

export function EmptyWithCTA() {
  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust {...props} items={[]} showTrustedDevicesCTA={true} />
    </ContextProvider>
  );
}

export function Processing() {
  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust {...props} attempt={{ status: 'processing' as any }} />
    </ContextProvider>
  );
}

export function Loaded() {
  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust {...props} />
    </ContextProvider>
  );
}

export function Failed() {
  return (
    <ContextProvider ctx={ctx}>
      <DeviceTrust
        {...props}
        attempt={{ status: 'failed', statusText: 'some error message' }}
      />
    </ContextProvider>
  );
}

const Devices: TrustedDevice[] = [
  {
    id: 'goteleport.local',
    assetTag: 'CSXXXXXXXXX',
    osType: 'macOS',
    enrollStatus: 'enrolled',
  },
  {
    id: 'goteleport.local',
    assetTag: 'DSXXXXXXXXX',
    osType: 'Linux',
    enrollStatus: 'not enrolled',
  },
  {
    id: 'goteleport.local',
    assetTag: 'ESXXXXXXXXX',
    osType: 'Windows',
    enrollStatus: 'enrolled',
  },
];

const props: State = {
  items: Devices,
  attempt: {
    status: 'success' as any,
  },
  fetchData: () => null,
  fetchStatus: 'disabled',
  startKey: '',
  showTrustedDevicesCTA: false,
};

const ctx = createTeleportContext();
