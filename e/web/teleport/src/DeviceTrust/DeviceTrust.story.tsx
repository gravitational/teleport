import React from 'react';

import { DeviceTrust } from './DeviceTrust';

import type { TrustedDevice } from 'e-teleport/services/devices/types';

import type { State } from './useDevices';

export default {
  title: 'TeleportE/DeviceTrust',
};

export function Empty() {
  return <DeviceTrust {...props} items={[]} />;
}

export function EmptyWithCTA() {
  return <DeviceTrust {...props} items={[]} showTrustedDevicesCTA={true} />;
}

export function Processing() {
  return <DeviceTrust {...props} attempt={{ status: 'processing' as any }} />;
}

export function Loaded() {
  return <DeviceTrust {...props} />;
}

export function Failed() {
  return (
    <DeviceTrust
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
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
