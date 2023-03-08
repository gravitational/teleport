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

import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import auth from 'teleport/services/auth';

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
        .getWebauthnResponse()
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
  onClose: () => void;
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
  onAuthenticated?: never;
};

// DefaultProps defines a function that
// accepts a privilegeTokenId that is only
// obtained after MFA response has been
// validated.
type DefaultProps = BaseProps & {
  onAuthenticated(privilegeTokenId: string): void;
  onMfaResponse?: never;
};

export type Props = MfaResponseProps | DefaultProps;

export type State = ReturnType<typeof useReAuthenticate>;
