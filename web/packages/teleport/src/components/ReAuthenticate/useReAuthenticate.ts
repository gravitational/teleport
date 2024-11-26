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

import { useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';
import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

import type {
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
} from 'teleport/services/mfa';

// useReAuthenticate will have different "submit" behaviors depending on:
//  - If prop field `onMfaResponse` is defined, after a user submits, the
//    function `onMfaResponse` is called with the user's MFA response.
//  - If prop field `onAuthenticated` is defined, after a user submits, the
//    user's MFA response are submitted with the request to get a privilege
//    token, and after successfully obtaining the token, the function
//    `onAuthenticated` will be called with this token.
export default function useReAuthenticate(props: Props) {
  const { onClose, actionText = defaultActionText } = props;

  // Note that attempt state "success" is not used or required.
  // After the user submits, the control is passed back
  // to the caller who is reponsible for rendering the `ReAuthenticate`
  // component.
  const { attempt, setAttempt } = useAttempt('');

  const [challenge, setMfaChallenge] = useState<MfaAuthenticateChallenge>(null);

  // Provide a custom error handler to catch a webauthn frontend error that occurs
  // on Firefox and replace it with a more helpful error message.
  const handleError = (err: Error) => {
    if (err.message.includes('attempt was made to use an object that is not')) {
      setAttempt({
        status: 'failed',
        statusText:
          'The two-factor device you used is not registered on this account. You must verify using a device that has already been registered.',
      });
      return;
    } else {
      setAttempt({ status: 'failed', statusText: err.message });
      return;
    }
  };

  // TODO(Joerger): Replace onAuthenticated with onMfaResponse at call sites (/e).
  if (props.onAuthenticated) {
    // Creating privilege tokens always expects the MANAGE_DEVICES webauthn scope.
    props.challengeScope = MfaChallengeScope.MANAGE_DEVICES;
    props.onMfaResponse = mfaResponse => {
      auth
        .createPrivilegeToken(mfaResponse)
        .then(props.onAuthenticated)
        .catch(handleError);
    };
  }

  async function getMfaChallenge() {
    if (challenge) {
      return challenge;
    }

    return auth.getMfaChallenge({ scope: props.challengeScope }).then(chal => {
      setMfaChallenge(chal);
      return chal;
    });
  }

  function clearMfaChallenge() {
    setMfaChallenge(null);
  }

  function getReauthMfaOptions() {
    return getMfaChallenge().then(createMfaOptions);
  }

  function submitWithMfa(mfaType?: Auth2faType, totp_code?: string) {
    setAttempt({ status: 'processing' });
    return getMfaChallenge()
      .then(chal => auth.getMfaChallengeResponse(chal, mfaType, totp_code))
      .then(props.onMfaResponse)
      .finally(clearMfaChallenge)
      .catch(handleError);
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  return {
    attempt,
    clearAttempt,
    getMfaChallenge,
    getReauthMfaOptions,
    submitWithMfa,
    actionText,
    onClose,
  };
}

const defaultActionText = 'performing this action';

export type Props = {
  onClose?: () => void;
  /**
   * The text that will be appended to the text in the re-authentication dialog.
   *
   * Default value: "performing this action"
   *
   * Example: If `actionText` is set to "registering a new device" then the dialog will say
   * "You must verify your identity with one of your existing two-factor devices before registering a new device."
   *
   * */
  actionText?: string;
  challengeScope?: MfaChallengeScope;
  onMfaResponse?(res: MfaChallengeResponse): void;
  // TODO(Joerger): Remove in favor of onMfaResponse, make onMfaResponse required.
  onAuthenticated?(privilegeTokenId: string): void;
};

export type State = ReturnType<typeof useReAuthenticate>;
