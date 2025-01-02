import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';
import RecoveryService from 'e-teleport/services/recovery';
import { RecoveryCodes } from 'teleport/services/auth';
import history from 'teleport/services/history';

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
