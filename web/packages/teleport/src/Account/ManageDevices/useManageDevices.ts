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

import cfg from 'teleport/config';
import { DeviceUsage, MfaDevice } from 'teleport/services/mfa';
import Ctx from 'teleport/teleportContext';

export default function useManageDevices(ctx: Ctx) {
  const [devices, setDevices] = useState<MfaDevice[]>([]);
  const [deviceToRemove, setDeviceToRemove] = useState<MfaDevice>();
  const fetchDevicesAttempt = useAttempt('');
  const [newDeviceUsage, setNewDeviceUsage] =
    useState<DeviceUsage>('passwordless');
  const [addDeviceWizardVisible, setAddDeviceWizardVisible] = useState(false);

  // This is a restricted privilege token that can only be used to add a device, in case
  // the user has no devices yet and thus can't authenticate using the ReAuthenticate dialog
  const createRestrictedTokenAttempt = useAttempt('');

  function fetchDevices() {
    fetchDevicesAttempt.run(() =>
      ctx.mfaService.fetchDevices().then(setDevices)
    );
  }

  async function onAddDevice(usage: DeviceUsage) {
    setNewDeviceUsage(usage);
    setAddDeviceWizardVisible(true);
  }

  function onDeviceAdded() {
    fetchDevices();
    setAddDeviceWizardVisible(false);
  }

  function onRemoveDevice(device: MfaDevice) {
    setDeviceToRemove(device);
  }

  function onDeviceRemoved() {
    fetchDevices();
    hideRemoveDevice();
  }

  function hideRemoveDevice() {
    setDeviceToRemove(null);
  }

  function closeAddDeviceWizard() {
    setAddDeviceWizardVisible(false);
  }

  useEffect(() => fetchDevices(), []);

  return {
    devices,
    onAddDevice,
    onRemoveDevice,
    onDeviceAdded,
    onDeviceRemoved,
    deviceToRemove,
    fetchDevicesAttempt: fetchDevicesAttempt.attempt,
    createRestrictedTokenAttempt: createRestrictedTokenAttempt.attempt,
    addDeviceWizardVisible,
    hideRemoveDevice,
    closeAddDeviceWizard,
    mfaDisabled: cfg.getAuth2faType() === 'off',
    newDeviceUsage,
  };
}

export type State = ReturnType<typeof useManageDevices>;
