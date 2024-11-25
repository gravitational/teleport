import api from 'teleport/services/api';
import { UrlResourcesParams } from 'teleport/config';
import { TrustedDevice } from 'teleport/DeviceTrust/types';
import { ResourcesResponse } from 'teleport/services/agents';

import cfg from 'e-teleport/config';

import { makeDevices } from './makeDevices';

// device fetch services
export const deviceService = {
  fetchDevices(params?: UrlResourcesParams) {
    return api.get(cfg.getTrustedDevicesUrl(params)).then(makeDevices);
  },
  fetchDevicesByUser(
    params: UrlResourcesParams,
    signal: AbortSignal
  ): Promise<ResourcesResponse<TrustedDevice>> {
    return api.get(cfg.getTrustedDevicesByUserUrl(params), signal);
  },
};
