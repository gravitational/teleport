import { useState, useEffect } from 'react';
import { useParams, generatePath } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import history from 'teleport/services/history';
import cfg from 'e-teleport/config';
import { RecoveryToken } from 'e-teleport/services/recovery/types';
import RecoveryService from 'e-teleport/services/recovery';

export default function useRecoveryFlow(recoveryService: RecoveryService) {
  const { tokenId } = useParams<{ tokenId: string }>();
  const [token, setToken] = useState<RecoveryToken>();
  const { attempt, run } = useAttempt('processing');

  useEffect(() => {
    run(() =>
      recoveryService.fetchRecoveryToken(tokenId).then(token => {
        setToken(token);
        if (!token.isApproved) {
          history.replace(
            generatePath(cfg.routes.recoveryStepVerify, { tokenId: token.id })
          );
        }
      })
    );
  }, [tokenId]);

  function goToNewCredential(token: RecoveryToken) {
    const path = token.isRecoverPassword
      ? cfg.routes.recoveryStepNewPassword
      : cfg.routes.recoveryStepNewDevice;
    history.push(generatePath(path, { tokenId: token.id }));
  }

  function goToDevices() {
    history.push(generatePath(cfg.routes.recoveryStepDevices, { tokenId }));
  }

  function goToCodes() {
    history.push(generatePath(cfg.routes.recoveryStepCodes, { tokenId }));
  }

  return {
    attempt,
    recoveryService,
    token,
    tokenId,
    goToNewCredential,
    goToDevices,
    goToCodes,
  };
}

export type State = ReturnType<typeof useRecoveryFlow>;
