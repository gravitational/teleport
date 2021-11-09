import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import history from 'teleport/services/history';
import { RecoveryCodes } from 'teleport/services/auth';
import RecoveryService from 'e-teleport/services/recovery';
import cfg from 'e-teleport/config';

export default function useCodes({ recoveryService, tokenId }: Props) {
  const { attempt, run } = useAttempt('processing');
  const [recoveryCodes, setRecoveryCodes] = useState<RecoveryCodes>();

  useEffect(() => {
    run(() =>
      recoveryService.generateRecoveryCodes(tokenId).then(setRecoveryCodes)
    );
  }, []);

  function redirect() {
    history.push(cfg.oss.routes.login, true);
  }

  return {
    attempt,
    recoveryCodes,
    redirect,
  };
}

export type State = ReturnType<typeof useCodes>;

export type Props = {
  recoveryService: RecoveryService;
  tokenId: string;
};
