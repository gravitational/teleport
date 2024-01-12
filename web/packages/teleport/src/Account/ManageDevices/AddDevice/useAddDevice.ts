/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import Ctx from 'teleport/teleportContext';
import authService from 'teleport/services/auth';
import { DeviceUsage } from 'teleport/services/mfa';
import cfg from 'teleport/config';

export default function useAddDevice(
  ctx: Ctx,
  { token, restrictDeviceUsage, fetchDevices, onClose }: Props
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
    restrictDeviceUsage,
  };
}

export type State = ReturnType<typeof useAddDevice>;

export type Props = {
  token: string;
  /**
   * Controls whether the user can customize whether the device should allow
   * passwordless authentication. `undefined` means that the user gets to
   * choose; other values mean that the component's call site decides what kind
   * of device we're adding.
   *
   * TODO(bl-nero): Disallow `undefined` when cleaning up the old flow.
   */
  restrictDeviceUsage?: DeviceUsage;
  fetchDevices: () => void;
  onClose: () => void;
};
