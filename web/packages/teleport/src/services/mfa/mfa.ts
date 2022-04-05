/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import auth, { makeWebauthnCreationResponse } from 'teleport/services/auth';
import {
  MfaDevice,
  AddNewTotpDeviceRequest,
  AddNewHardwareDeviceRequest,
} from './types';
import makeMfaDevice from './makeMfaDevice';

class MfaService {
  fetchDevicesWithToken(tokenId: string): Promise<MfaDevice[]> {
    return api
      .get(cfg.getMfaDevicesWithTokenUrl(tokenId))
      .then(devices => devices.map(makeMfaDevice));
  }

  removeDevice(tokenId: string, deviceName: string) {
    return api.delete(cfg.getMfaDeviceUrl(tokenId, deviceName));
  }

  fetchDevices(): Promise<MfaDevice[]> {
    return api
      .get(cfg.api.mfaDevicesPath)
      .then(devices => devices.map(makeMfaDevice));
  }

  addNewTotpDevice(req: AddNewTotpDeviceRequest) {
    return api.post(cfg.api.mfaDevicesPath, req);
  }

  addNewWebauthnDevice(req: AddNewHardwareDeviceRequest) {
    return auth
      .checkWebauthnSupport()
      .then(() => auth.createMfaRegistrationChallenge(req.tokenId, 'webauthn'))
      .then(res =>
        navigator.credentials.create({
          publicKey: res.webauthnPublicKey,
        })
      )
      .then(res => {
        const request = {
          ...req,
          webauthnRegisterResponse: makeWebauthnCreationResponse(res),
        };

        return api.post(cfg.api.mfaDevicesPath, request);
      });
  }
}

export default MfaService;
