import api from 'teleport/services/api';
import { UrlResourcesParams } from 'teleport/config';

import cfg from 'e-teleport/config';

import { makeDevices } from './makeDevices';

// device fetch services
export const deviceService = {
  fetchDevices(params?: UrlResourcesParams) {
    return api.get(cfg.getTrustedDevicesUrl(params)).then(makeDevices);
  },
};
