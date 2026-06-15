import useAttempt from 'shared/hooks/useAttemptNext';

import RecoveryService from 'e-teleport/services/recovery';
import {
  RecoveryToken,
  VerifyUserRequest,
} from 'e-teleport/services/recovery/types';
import auth from 'teleport/services/auth';
import { DeviceType, MfaAuthenticateChallenge } from 'teleport/services/mfa';

export default function useVerifyUser({ recoveryService, token, done }: Props) {
  const { attempt: submitAttempt, setAttempt, handleError } = useAttempt('');

  function submitWithPassword(password: string) {
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

  function submitWithMfa(
    challenge: MfaAuthenticateChallenge,
    mfaType: DeviceType,
    totpCode?: string
  ) {
    setAttempt({ status: 'processing' });
    auth
      .getMfaChallengeResponse(challenge, mfaType, totpCode)
      .then(response => {
        const req: VerifyUserRequest = {
          tokenId: token.id,
          username: token.username,
        };
        // Account recovery doesn't apply to SSO users, so the challenge only
        // ever offers TOTP or WebAuthn. SSO MFA (response.sso_response) is
        // intentionally not handled here, and the backend doesn't accept it for
        // recovery either.
        if (response.totp_code) {
          req.secondFactorToken = response.totp_code;
        } else if (response.webauthn_response) {
          req.webauthnAssertionResponse = response.webauthn_response;
        }
        return recoveryService.verifyUser(req);
      })
      .then(done)
      .catch(handleError);
  }

  return {
    token,
    submitAttempt,
    submitWithPassword,
    submitWithMfa,
  };
}

export type State = ReturnType<typeof useVerifyUser>;
export type Props = {
  recoveryService: RecoveryService;
  token: RecoveryToken;
  done: (token: RecoveryToken) => void;
};
