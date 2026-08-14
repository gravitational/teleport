import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContextE from 'e-teleport/teleportContextE';
import { RecoveryCodes } from 'teleport/services/auth';

export default function useRecoveryCodesDialog(
  ctx: TeleportContextE,
  { token, close, refreshDate, isNewCodes }: Props
) {
  const [recoveryCodes, setRecoveryCodes] = useState<RecoveryCodes>();
  const { attempt, run } = useAttempt('');

  function generateCodes() {
    run(() =>
      ctx.recoveryService.generateRecoveryCodes(token).then(setRecoveryCodes)
    );
  }

  function closeWithDateRefresh() {
    refreshDate();
    close();
  }

  useEffect(() => generateCodes(), []);

  return {
    attempt,
    recoveryCodes,
    closeWithDateRefresh,
    close,
    generateCodes,
    isNewCodes,
  };
}

export type Props = {
  token: string;
  close: () => void;
  refreshDate: () => void;
  isNewCodes: boolean;
};

export type State = ReturnType<typeof useRecoveryCodesDialog>;
