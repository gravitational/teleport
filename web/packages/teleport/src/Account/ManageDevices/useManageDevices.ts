/*
Copyright 2021 Gravitational, Inc.

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

import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import { MfaDevice } from 'teleport/services/mfa';

export default function useManageDevices(ctx: Ctx) {
  const [devices, setDevices] = useState<MfaDevice[]>([]);
  const [isDialogVisible, setIsDialogVisible] = useState(false);
  const [deviceToRemove, setDeviceToRemove] = useState<DeviceToRemove>();
  const [token, setToken] = useState('');
  const fetchDevicesAttempt = useAttempt('');

  // This is a restricted privilege token that can only be used to add a device, in case
  // the user has no devices yet and thus can't authenticate using the ReAuthenticate dialog
  const createRestrictedTokenAttempt = useAttempt('');

  const isReAuthenticateVisible = !token && isDialogVisible;
  const isRemoveDeviceVisible = token && deviceToRemove && isDialogVisible;
  const isAddDeviceVisible = token && !deviceToRemove && isDialogVisible;

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

  function onAddDevice() {
    if (devices.length === 0) {
      createRestrictedTokenAttempt.run(() =>
        auth.createRestrictedPrivilegeToken().then(token => {
          setToken(token);
          setIsDialogVisible(true);
        })
      );
    } else {
      setIsDialogVisible(true);
    }
  }

  function hideAddDevice() {
    setIsDialogVisible(false);
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

  useEffect(() => fetchDevices(), []);

  return {
    devices,
    token,
    setToken,
    onAddDevice,
    onRemoveDevice,
    deviceToRemove,
    fetchDevices,
    removeDevice,
    fetchDevicesAttempt: fetchDevicesAttempt.attempt,
    createRestrictedTokenAttempt: createRestrictedTokenAttempt.attempt,
    isReAuthenticateVisible,
    isAddDeviceVisible,
    isRemoveDeviceVisible,
    hideReAuthenticate,
    hideAddDevice,
    hideRemoveDevice,
    mfaDisabled: cfg.getAuth2faType() === 'off',
  };
}

type DeviceToRemove = Pick<MfaDevice, 'id' | 'name'>;

export type State = ReturnType<typeof useManageDevices>;
