/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import history from 'teleport/services/history';
import auth, {
  ChangedUserAuthn,
  RecoveryCodes,
  ResetPasswordReqWithEvent,
  ResetPasswordWithWebauthnReqWithEvent,
  ResetToken,
} from 'teleport/services/auth';
import { UseTokenState } from 'teleport/Welcome/NewCredentials/types';

export default function useToken(tokenId: string): UseTokenState {
  const [resetToken, setResetToken] = useState<ResetToken>();
  const [recoveryCodes, setRecoveryCodes] = useState<RecoveryCodes>();
  const [success, setSuccess] = useState(false); // TODO rename
  const [privateKeyPolicyEnabled, setPrivateKeyPolicyEnabled] = useState(false);

  const fetchAttempt = useAttempt('');
  const submitAttempt = useAttempt('');
  const auth2faType = cfg.getAuth2faType();

  useEffect(() => {
    fetchAttempt.run(() =>
      auth
        .fetchPasswordToken(tokenId)
        .then(resetToken => setResetToken(resetToken))
    );
  }, []);

  function handleResponse(res: ChangedUserAuthn) {
    if (res.privateKeyPolicyEnabled) {
      setPrivateKeyPolicyEnabled(true);
    }
    if (res.recovery.createdDate) {
      setRecoveryCodes(res.recovery);
    } else {
      finishedRegister();
    }
  }

  function onSubmit(password: string, otpCode = '', deviceName = '') {
    const req: ResetPasswordReqWithEvent = {
      req: { tokenId, password, otpCode, deviceName },
      eventMeta: { username: resetToken.user },
    };

    submitAttempt.setAttempt({ status: 'processing' });
    auth
      .resetPassword(req)
      .then(handleResponse)
      .catch(submitAttempt.handleError);
  }

  function onSubmitWithWebauthn(password?: string, deviceName = '') {
    const req: ResetPasswordWithWebauthnReqWithEvent = {
      req: { tokenId, password, deviceName },
      eventMeta: { username: resetToken.user, mfaType: auth2faType },
    };

    submitAttempt.setAttempt({ status: 'processing' });
    auth
      .resetPasswordWithWebauthn(req)
      .then(handleResponse)
      .catch(submitAttempt.handleError);
  }

  function redirect() {
    history.push(cfg.routes.root, true);
  }

  function clearSubmitAttempt() {
    submitAttempt.setAttempt({ status: '' });
  }

  function finishedRegister() {
    setSuccess(true);
  }

  return {
    auth2faType,
    primaryAuthType: cfg.getPrimaryAuthType(),
    isPasswordlessEnabled: cfg.isPasswordlessEnabled(),
    fetchAttempt: fetchAttempt.attempt,
    submitAttempt: submitAttempt.attempt,
    clearSubmitAttempt,
    onSubmit,
    onSubmitWithWebauthn,
    resetToken,
    recoveryCodes,
    redirect,
    success,
    finishedRegister,
    privateKeyPolicyEnabled,
  };
}
