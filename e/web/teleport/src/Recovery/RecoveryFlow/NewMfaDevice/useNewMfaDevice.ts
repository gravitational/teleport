import useAttempt from 'shared/hooks/useAttemptNext';
import cfg from 'e-teleport/config';
import RecoveryService from 'e-teleport/services/recovery';

export default function useNewMfaDevice({
  recoveryService,
  tokenId,
  qrCode,
  onNext,
}: Props) {
  const { attempt, setAttempt, handleError } = useAttempt('');
  const auth2faType = cfg.oss.getAuth2faType();

  function setNewTotpDevice(secondFactorToken: string, deviceName: string) {
    setAttempt({ status: 'processing' });
    recoveryService
      .setNewTotpDeviceOrPassword({
        tokenId,
        secondFactorToken,
        deviceName,
      })
      .then(onNext)
      .catch(handleError);
  }

  function setNewU2fDevice(deviceName: string) {
    setAttempt({ status: 'processing' });
    recoveryService
      .setNewU2fDevice({ tokenId, deviceName })
      .then(onNext)
      .catch(handleError);
  }

  function setNewWebauthnDevice(deviceName: string) {
    setAttempt({ status: 'processing' });
    recoveryService
      .setNewWebauthnDevice({ tokenId, deviceName })
      .then(onNext)
      .catch(handleError);
  }

  function clearSubmitAttempt() {
    setAttempt({ status: '' });
  }

  return {
    attempt,
    clearSubmitAttempt,
    setNewTotpDevice,
    setNewU2fDevice,
    setNewWebauthnDevice,
    qrCode,
    auth2faType,
    preferredMfaType: cfg.oss.getPreferredMfaType(),
  };
}

export type State = ReturnType<typeof useNewMfaDevice>;

export type Props = {
  recoveryService: RecoveryService;
  onNext: () => void;
  tokenId: string;
  qrCode: string;
};
