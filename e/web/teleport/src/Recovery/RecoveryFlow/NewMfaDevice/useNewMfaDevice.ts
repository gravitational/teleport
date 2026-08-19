import { useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';
import RecoveryService from 'e-teleport/services/recovery';
import auth from 'teleport/services/auth/auth';

export default function useNewMfaDevice({
  recoveryService,
  tokenId,
  qrCode,
  onNext,
}: Props) {
  const submitAttempt = useAttempt('');
  const auth2faType = cfg.oss.getAuth2faType();
  const [credential, setCredential] = useState<Credential | undefined>();

  function createNewWebAuthnDevice() {
    submitAttempt.run(async () => {
      setCredential(
        await auth.createNewWebAuthnDevice({ tokenId, deviceUsage: 'mfa' })
      );
    });
  }

  function setNewTotpDevice(otpCode: string, deviceName: string) {
    submitAttempt.run(async () => {
      await recoveryService.setNewTotpDeviceOrPassword({
        tokenId,
        otpCode,
        deviceName,
      });
      onNext();
    });
  }

  function setNewWebauthnDevice(deviceName: string) {
    submitAttempt.run(async () => {
      await recoveryService.setNewWebauthnDevice({
        credentialRequest: { tokenId, deviceName },
        credential,
      });
      onNext();
    });
  }

  function clearSubmitAttempt() {
    submitAttempt.setAttempt({ status: '' });
    setCredential(undefined);
  }

  return {
    attempt: submitAttempt.attempt,
    clearSubmitAttempt,
    createNewWebAuthnDevice,
    setNewTotpDevice,
    setNewWebauthnDevice,
    credential,
    qrCode,
    auth2faType,
    preferredMfaType: cfg.oss.getPreferredMfaType(),
  };
}

export type Props = {
  recoveryService: RecoveryService;
  onNext: () => void;
  tokenId: string;
  qrCode: string;
};
