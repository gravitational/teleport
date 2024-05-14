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

import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';
import auth, { DeviceUsage } from 'teleport/services/auth';
import { MfaDevice } from 'teleport/services/mfa';

export default function useManageDevices(ctx: Ctx) {
  const [devices, setDevices] = useState<MfaDevice[]>([]);
  const [isDialogVisible, setIsDialogVisible] = useState(false);
  const [deviceToRemove, setDeviceToRemove] = useState<DeviceToRemove>();
  const [token, setToken] = useState('');
  const fetchDevicesAttempt = useAttempt('');
  const [newDeviceUsage, setNewDeviceUsage] =
    useState<DeviceUsage>('passwordless');
  const [addDeviceWizardVisible, setAddDeviceWizardVisible] = useState(false);

  // This is a restricted privilege token that can only be used to add a device, in case
  // the user has no devices yet and thus can't authenticate using the ReAuthenticate dialog
  const createRestrictedTokenAttempt = useAttempt('');

  const isReAuthenticateVisible = !token && isDialogVisible;
  const isRemoveDeviceVisible = token && deviceToRemove && isDialogVisible;
  const isReauthenticationRequired = !token;

  function fetchDevices() {
    fetchDevicesAttempt.run(() =>
      ctx.mfaService.fetchDevices().then(setDevices)
    );
  }

  function removeDevice() {
    return ctx.mfaService.removeDevice(token, deviceToRemove.name).then(() => {
      fetchDevices();
      hideRemoveDevice();
    });
  }

  function onAddDevice(usage: DeviceUsage) {
    setNewDeviceUsage(usage);
    if (devices.length === 0) {
      createRestrictedTokenAttempt.run(() =>
        auth.createRestrictedPrivilegeToken().then(token => {
          setToken(token);
          setAddDeviceWizardVisible(true);
        })
      );
    } else {
      setAddDeviceWizardVisible(true);
    }
  }

  function onDeviceAdded() {
    fetchDevices();
    setAddDeviceWizardVisible(false);
    setToken(null);
  }

  function onRemoveDevice(device: DeviceToRemove) {
    setDeviceToRemove(device);
    setIsDialogVisible(true);
  }

  function hideRemoveDevice() {
    setIsDialogVisible(false);
    setDeviceToRemove(null);
    setToken(null);
  }

  function hideReAuthenticate() {
    setIsDialogVisible(false);
  }

  function closeAddDeviceWizard() {
    setAddDeviceWizardVisible(false);
  }

  useEffect(() => fetchDevices(), []);

  return {
    devices,
    token,
    setToken,
    onAddDevice,
    onRemoveDevice,
    onDeviceAdded,
    deviceToRemove,
    removeDevice,
    fetchDevicesAttempt: fetchDevicesAttempt.attempt,
    createRestrictedTokenAttempt: createRestrictedTokenAttempt.attempt,
    isReAuthenticateVisible,
    isRemoveDeviceVisible,
    isReauthenticationRequired,
    addDeviceWizardVisible,
    hideReAuthenticate,
    hideRemoveDevice,
    closeAddDeviceWizard,
    mfaDisabled: cfg.getAuth2faType() === 'off',
    newDeviceUsage,
  };
}

type DeviceToRemove = Pick<MfaDevice, 'id' | 'name'>;

export type State = ReturnType<typeof useManageDevices>;
