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

import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

import type { MfaAuthnResponse } from 'teleport/services/mfa';

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
  const { attempt, setAttempt, handleError } = useAttempt('');

  function submitWithTotp(secondFactorToken: string) {
    if ('onMfaResponse' in props) {
      props.onMfaResponse({ totp_code: secondFactorToken });
      return;
    }

    setAttempt({ status: 'processing' });
    auth
      .createPrivilegeTokenWithTotp(secondFactorToken)
      .then(props.onAuthenticated)
      .catch(handleError);
  }

  function submitWithWebauthn() {
    setAttempt({ status: 'processing' });

    if ('onMfaResponse' in props) {
      auth
        .getWebauthnResponse(props.challengeScope)
        .then(webauthnResponse =>
          props.onMfaResponse({ webauthn_response: webauthnResponse })
        )
        .catch(handleError);
      return;
    }

    auth
      .createPrivilegeTokenWithWebauthn()
      .then(props.onAuthenticated)
      .catch((err: Error) => {
        // This catches a webauthn frontend error that occurs on Firefox and replaces it with a more helpful error message.
        if (
          err.message.includes('attempt was made to use an object that is not')
        ) {
          setAttempt({
            status: 'failed',
            statusText:
              'The two-factor device you used is not registered on this account. You must verify using a device that has already been registered.',
          });
        } else {
          setAttempt({ status: 'failed', statusText: err.message });
        }
      });
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
    actionText,
    onClose,
  };
}

const defaultActionText = 'performing this action';

type BaseProps = {
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
};

// MfaResponseProps defines a function
// that accepts a MFA response. No
// authentication has been done at this point.
type MfaResponseProps = BaseProps & {
  onMfaResponse(res: MfaAuthnResponse): void;
  /**
   * The MFA challenge scope of the action to perform, as defined in webauthn.proto.
   */
  challengeScope: MfaChallengeScope;
  onAuthenticated?: never;
};

// DefaultProps defines a function that
// accepts a privilegeTokenId that is only
// obtained after MFA response has been
// validated.
type DefaultProps = BaseProps & {
  onAuthenticated(privilegeTokenId: string): void;
  onMfaResponse?: never;
  challengeScope?: never;
};

export type Props = MfaResponseProps | DefaultProps;

export type State = ReturnType<typeof useReAuthenticate>;
