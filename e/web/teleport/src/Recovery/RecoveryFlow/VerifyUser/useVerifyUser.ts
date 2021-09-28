import useAttempt from 'shared/hooks/useAttemptNext';
import RecoveryService from 'e-teleport/services/recovery';
import { RecoveryToken } from 'e-teleport/services/recovery/types';
import cfg from 'e-teleport/config';

export default function useVerifyUser({ recoveryService, token, done }: Props) {
  const { attempt, setAttempt, handleError } = useAttempt('');
  const auth2faType = cfg.oss.getAuth2faType();

  function submitPasswordCreds(password: string) {
    setAttempt({ status: 'processing' });
    recoveryService
      .verifyUser({
        tokenId: token.id,
        username: token.username,
        password,
      })
      .then(done)
      .catch(handleError);
  }

  function submitTotpCreds(secondFactorToken: string) {
    setAttempt({ status: 'processing' });
    recoveryService
      .verifyUser({
        tokenId: token.id,
        username: token.username,
        secondFactorToken,
      })
      .then(done)
      .catch(handleError);
  }

  function submitU2fCreds() {
    setAttempt({ status: 'processing' });
    recoveryService
      .verifyUserWithU2f(token.id, token.username)
      .then(done)
      .catch(handleError);
  }

  return {
    attempt,
    token,
    submitPasswordCreds,
    submitTotpCreds,
    submitU2fCreds,
    auth2faType,
  };
}

export type State = ReturnType<typeof useVerifyUser>;
export type Props = {
  recoveryService: RecoveryService;
  token: RecoveryToken;
  done: (token: RecoveryToken) => void;
};
