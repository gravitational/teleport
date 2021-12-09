import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import cfg from 'e-teleport/config';
import history from 'teleport/services/history';
import auth, { RecoveryCodes } from 'teleport/services/auth';

// TODO (alex-kovoy): refactor it to reuse useUserTokenOSS
export default function useToken(tokenId: string) {
  const [passwordToken, setPswToken] = useState<ResetToken>(undefined);
  const [recoveryCodes, setRecoveryCodes] = useState<RecoveryCodes>();
  const fetchAttempt = useAttempt('');
  const submitAttempt = useAttempt('');
  const auth2faType = cfg.oss.getAuth2faType();

  useEffect(() => {
    fetchAttempt.run(() =>
      auth
        .fetchPasswordToken(tokenId)
        .then(resetToken => setPswToken(resetToken))
    );
  }, []);

  function onSubmit(password: string, otpToken: string) {
    submitAttempt.setAttempt({ status: 'processing' });
    auth
      .resetPassword(tokenId, password, otpToken)
      .then(recoveryCodes => {
        if (recoveryCodes.createdDate) {
          setRecoveryCodes(recoveryCodes);
        } else {
          redirect();
        }
      })
      .catch(submitAttempt.handleError);
  }

  function onSubmitWithU2f(password: string) {
    submitAttempt.setAttempt({ status: 'processing' });
    auth
      .resetPasswordWithU2f(tokenId, password)
      .then(recoveryCodes => {
        if (recoveryCodes.createdDate) {
          setRecoveryCodes(recoveryCodes);
        } else {
          redirect();
        }
      })
      .catch(submitAttempt.handleError);
  }

  function onSubmitWithWebauthn(password: string) {
    submitAttempt.setAttempt({ status: 'processing' });
    auth
      .resetPasswordWithWebauthn(tokenId, password)
      .then(recoveryCodes => {
        if (recoveryCodes.createdDate) {
          setRecoveryCodes(recoveryCodes);
        } else {
          redirect();
        }
      })
      .catch(submitAttempt.handleError);
  }

  function redirect() {
    history.push(cfg.oss.routes.root, true);
  }

  function clearSubmitAttempt() {
    submitAttempt.setAttempt({ status: '' });
  }

  return {
    auth2faType,
    preferredMfaType: cfg.oss.getPreferredMfaType(),
    fetchAttempt: fetchAttempt.attempt,
    submitAttempt: submitAttempt.attempt,
    clearSubmitAttempt,
    onSubmit,
    onSubmitWithU2f,
    onSubmitWithWebauthn,
    passwordToken,
    recoveryCodes,
    redirect,
  };
}

type ResetToken = {
  tokenId: string;
  qrCode: string;
  user: string;
};

export type State = ReturnType<typeof useToken>;
