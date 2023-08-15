import { TrustedDeviceResponse } from 'teleport/DeviceTrust/types';

// response parsing, filtering should go here
export const makeDevices = (json): TrustedDeviceResponse => {
  const { items, startKey } = json;

  return { items: items ?? [], startKey };
};
