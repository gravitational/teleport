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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import Ctx from 'teleport/teleportContext';
import authService from 'teleport/services/auth';
import { DeviceUsage } from 'teleport/services/mfa';
import cfg from 'teleport/config';

export default function useAddDevice(
  ctx: Ctx,
  { token, fetchDevices, onClose }: Props
) {
  const [qrCode, setQrCode] = useState('');
  const addDeviceAttempt = useAttempt('');
  const fetchQrCodeAttempt = useAttempt('');

  function addTotpDevice(secondFactorToken: string, deviceName: string) {
    addDeviceAttempt.setAttempt({ status: 'processing' });
    ctx.mfaService
      .addNewTotpDevice({
        tokenId: token,
        secondFactorToken,
        deviceName,
      })
      .then(() => {
        onClose();
        fetchDevices();
      })
      .catch(addDeviceAttempt.handleError);
  }

  function addWebauthnDevice(deviceName: string, deviceUsage: DeviceUsage) {
    addDeviceAttempt.setAttempt({ status: 'processing' });
    ctx.mfaService
      .addNewWebauthnDevice({
        tokenId: token,
        deviceName,
        deviceUsage,
      })
      .then(() => {
        onClose();
        fetchDevices();
      })
      .catch(addDeviceAttempt.handleError);
  }

  function clearAttempt() {
    addDeviceAttempt.setAttempt({ status: '' });
  }

  useEffect(() => {
    fetchQrCodeAttempt.run(() =>
      authService
        .createMfaRegistrationChallenge(token, 'totp')
        .then(res => setQrCode(res.qrCode))
    );
  }, []);

  return {
    addDeviceAttempt: addDeviceAttempt.attempt,
    fetchQrCodeAttempt: fetchQrCodeAttempt.attempt,
    addTotpDevice,
    addWebauthnDevice,
    onClose,
    clearAttempt,
    qrCode,
    auth2faType: cfg.getAuth2faType(),
    isPasswordlessEnabled: cfg.isPasswordlessEnabled(),
  };
}

export type State = ReturnType<typeof useAddDevice>;

export type Props = {
  token: string;
  fetchDevices: () => void;
  onClose: () => void;
};
