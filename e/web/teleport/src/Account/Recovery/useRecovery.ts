import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContextE from 'e-teleport/teleportContextE';
import { isValidEmail } from 'e-teleport/validations/email';

export default function useRecovery(
  ctx: TeleportContextE,
  onError?: (statusText: string) => void
) {
  const [token, setToken] = useState('');
  const [createdDate, setCreatedDate] = useState<Date | undefined>();
  const [isDialogVisible, setIsDialogVisible] = useState(false);
  const { attempt, run } = useAttempt('');

  const userHasCodes = !!createdDate;
  const createdDateText = userHasCodes ? createdDate.toLocaleDateString() : '';

  const isRecoveryEnabled =
    isValidEmail(ctx.storeUser.getUsername()) && !ctx.storeUser.isSso();

  const isReAuthenticateVisible = !token && isDialogVisible;
  const isCodesVisible = token && isDialogVisible;

  function showReAuthenticate() {
    setIsDialogVisible(true);
  }

  function hideReAuthenticate() {
    setIsDialogVisible(false);
  }

  function hideCodes() {
    setIsDialogVisible(false);
    setToken(null);
  }

  function fetchCreatedDate() {
    run(async () => {
      try {
        const metadata = await ctx.recoveryService.fetchRecoveryCodesMetadata();
        setCreatedDate(metadata.createdDate);
      } catch (e) {
        onError?.(e.message);
        throw e;
      }
    });
  }

  useEffect(() => fetchCreatedDate(), []);

  return {
    attempt,
    token,
    setToken,
    createdDateText,
    showReAuthenticate,
    hideReAuthenticate,
    isReAuthenticateVisible,
    hideCodes,
    isCodesVisible,
    fetchCreatedDate,
    userHasCodes,
    isRecoveryEnabled,
  };
}

export type State = ReturnType<typeof useRecovery>;
