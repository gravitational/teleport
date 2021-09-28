import cfg from 'teleport/config';
import api from 'teleport/services/api';
import makeMfaDevice from './makeMfaDevice';
import { MfaDevice } from './types';

class MfaService {
  fetchDevicesWithToken(tokenId: string): Promise<MfaDevice[]> {
    return api
      .get(cfg.getMfaDeviceListUrl(tokenId))
      .then(devices => devices.map(makeMfaDevice));
  }

  removeDevice(tokenId: string, deviceName: string) {
    return api.delete(cfg.getMfaDeviceUrl(tokenId, deviceName));
  }
}

export default MfaService;
