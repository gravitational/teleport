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
  const [token, setToken] = useState('');
  const fetchDevicesAttempt = useAttempt('');
  const [newDeviceUsage, setNewDeviceUsage] =
    useState<DeviceUsage>('passwordless');
  const [addDeviceWizardVisible, setAddDeviceWizardVisible] = useState(false);

  // This is a restricted privilege token that can only be used to add a device, in case
  // the user has no devices yet and thus can't authenticate using the ReAuthenticate dialog
  const createRestrictedTokenAttempt = useAttempt('');

  const isReauthenticationRequired = !token;

  function fetchDevices() {
    fetchDevicesAttempt.run(() =>
      ctx.mfaService.fetchDevices().then(setDevices)
    );
  }

  async function onAddDevice(usage: DeviceUsage) {
    setNewDeviceUsage(usage);
    const challenge = await auth.getMfaChallenge({
      scope: MfaChallengeScope.MANAGE_DEVICES,
    });
    // If the user doesn't receieve any challenges from the backend, that means
    // they have no valid devices to be challenged and should instead use a privilege token
    // to add a new device.
    // TODO (avatus): add SSO challenge here as well when we add SSO for MFA
    // TODO(Joerger): privilege token is no longer required to add first device.
    if (!challenge) {
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
    token,
    onAddDevice,
    onRemoveDevice,
    onDeviceAdded,
    onDeviceRemoved,
    deviceToRemove,
    fetchDevicesAttempt: fetchDevicesAttempt.attempt,
    createRestrictedTokenAttempt: createRestrictedTokenAttempt.attempt,
    isReauthenticationRequired,
    addDeviceWizardVisible,
    hideRemoveDevice,
    closeAddDeviceWizard,
    mfaDisabled: cfg.getAuth2faType() === 'off',
    newDeviceUsage,
  };
}

export type State = ReturnType<typeof useManageDevices>;
