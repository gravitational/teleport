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

import React from 'react';
import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function useReAuthenticate({ onAuthenticated, onClose }: Props) {
  const { attempt, setAttempt, handleError } = useAttempt('');

  function submitWithTotp(secondFactorToken: string) {
    setAttempt({ status: 'processing' });
    auth
      .createPrivilegeTokenWithTotp(secondFactorToken)
      .then(onAuthenticated)
      .catch(handleError);
  }

  function submitWithWebauthn() {
    setAttempt({ status: 'processing' });
    auth
      .createPrivilegeTokenWithWebauthn()
      .then(onAuthenticated)
      .catch(handleError);
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  return {
    attempt,
    clearAttempt,
    submitWithTotp,
    submitWithWebauthn,
    auth2faType: cfg.getAuth2faType(),
    preferredMfaType: cfg.getPreferredMfaType(),
    onClose,
  };
}

export type Props = {
  onAuthenticated: React.Dispatch<React.SetStateAction<string>>;
  onClose: () => void;
};

export type State = ReturnType<typeof useReAuthenticate>;
