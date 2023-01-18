import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import MfaService, { MfaDevice } from 'teleport/services/mfa';

export default function useDevices(
  { tokenId, onNext }: Props,
  mfaService: MfaService
) {
  const { attempt, run } = useAttempt('processing');
  const [devices, setDevices] = useState<MfaDevice[]>([]);
  const [deviceToRemove, setDeviceToRemove] = useState<DeviceToRemove>();

  useEffect(() => {
    run(() => mfaService.fetchDevicesWithToken(tokenId).then(setDevices));
  }, []);

  function removeDevice() {
    return mfaService.removeDevice(tokenId, deviceToRemove.name).then(() => {
      const tmp = devices.filter(device => device.id !== deviceToRemove.id);
      setDevices(tmp);
      closeDialog();
    });
  }

  function closeDialog() {
    setDeviceToRemove(null);
  }

  return {
    attempt,
    devices,
    removeDevice,
    onNext,
    closeDialog,
    deviceToRemove,
    setDeviceToRemove,
  };
}

export type State = ReturnType<typeof useDevices>;

export type Props = {
  onNext: () => void;
  tokenId: string;
};

export type DeviceToRemove = Pick<MfaDevice, 'id' | 'name'>;
