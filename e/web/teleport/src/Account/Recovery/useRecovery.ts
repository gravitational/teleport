import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';

export default function useRecovery(ctx: TeleportContextE) {
  const [token, setToken] = useState('');
  const [createdDate, setCreatedDate] = useState<Date>();
  const [isDialogVisible, setIsDialogVisible] = useState(false);
  const { attempt, run } = useAttempt('');

  const userHasCodes = !!createdDate;
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
    run(() =>
      ctx.recoveryService
        .fetchRecoveryCodesMetadata()
        .then(metadata => setCreatedDate(metadata.createdDate))
    );
  }

  useEffect(() => fetchCreatedDate(), []);

  return {
    attempt,
    token,
    setToken,
    createdDate,
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

// isValidEmail returns true if the string is a valid email eg. is formatted as chars@chars{at least one dot}chars
function isValidEmail(email: string) {
  const emailParts = email.split('@');
  return (
    emailParts.length === 2 &&
    emailParts[0] &&
    emailParts[1] &&
    emailParts[1].indexOf('.') !== -1
  );
}

export type State = ReturnType<typeof useRecovery>;
