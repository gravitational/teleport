import useAttempt from 'shared/hooks/useAttemptNext';

import RecoveryService from 'e-teleport/services/recovery';

export default function useNewPassword({
  recoveryService,
  tokenId,
  onNext,
}: Props) {
  const { attempt, setAttempt, handleError } = useAttempt('');

  function setNewPassword(password: string) {
    setAttempt({ status: 'processing' });
    recoveryService
      .setNewTotpDeviceOrPassword({ tokenId, password })
      .then(onNext)
      .catch(handleError);
  }

  return {
    attempt,
    setNewPassword,
  };
}

export type State = ReturnType<typeof useNewPassword>;
export type Props = {
  recoveryService: RecoveryService;
  onNext: () => void;
  tokenId: string;
};
