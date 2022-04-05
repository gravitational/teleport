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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import auth from 'teleport/services/auth';

export default function useToken(tokenId: string) {
  const [passwordToken, setPswToken] = useState<ResetToken>();
  const fetchAttempt = useAttempt('');
  const submitAttempt = useAttempt('');
  const auth2faType = cfg.getAuth2faType();

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
      .then(redirect)
      .catch(submitAttempt.handleError);
  }

  function onSubmitWithWebauthn(password: string) {
    submitAttempt.setAttempt({ status: 'processing' });
    auth
      .resetPasswordWithWebauthn(tokenId, password)
      .then(redirect)
      .catch(submitAttempt.handleError);
  }

  function redirect() {
    history.push(cfg.routes.root, true);
  }

  function clearSubmitAttempt() {
    submitAttempt.setAttempt({ status: '' });
  }

  return {
    auth2faType,
    preferredMfaType: cfg.getPreferredMfaType(),
    fetchAttempt: fetchAttempt.attempt,
    submitAttempt: submitAttempt.attempt,
    clearSubmitAttempt,
    onSubmit,
    onSubmitWithWebauthn,
    passwordToken,
  };
}

type ResetToken = {
  tokenId: string;
  qrCode: string;
  user: string;
};

export type State = ReturnType<typeof useToken>;
