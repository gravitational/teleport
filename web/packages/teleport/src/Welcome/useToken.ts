/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect, useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import history from 'teleport/services/history';
import auth, {
  ChangedUserAuthn,
  DeviceUsage,
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
  const [credential, setCredential] = useState<Credential | undefined>();

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

  function createNewWebAuthnDevice(deviceUsage: DeviceUsage) {
    submitAttempt.run(async () => {
      setCredential(
        await auth.createNewWebAuthnDevice({ tokenId, deviceUsage })
      );
    });
  }

  function onSubmitWithWebauthn(password?: string, deviceName = '') {
    const req: ResetPasswordWithWebauthnReqWithEvent = {
      req: { tokenId, password, deviceName },
      credential,
      eventMeta: { username: resetToken.user, mfaType: auth2faType },
    };
    submitAttempt.run(async () => {
      handleResponse(await auth.resetPasswordWithWebauthn(req));
    });
  }

  function redirect() {
    history.push(cfg.routes.root, true);
  }

  function clearSubmitAttempt() {
    submitAttempt.setAttempt({ status: '' });
    setCredential(undefined);
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
    credential,
    clearSubmitAttempt,
    onSubmit,
    createNewWebAuthnDevice, // Added missing property
    onSubmitWithWebauthn,
    resetToken,
    recoveryCodes,
    redirect,
    success,
    finishedRegister,
  };
}
