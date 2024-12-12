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

import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

import {
  DeviceType,
  DeviceUsage,
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
} from 'teleport/services/mfa';

export default function useReAuthenticate(props: ReauthProps): ReauthState {
  async function getMfaChallenge() {
    const [[challenge], error] = await getChallenge();
    if (error) throw error;
    return challenge;
  }

  const [getChallengeAttempt, getChallenge, setGetChallengeAttempt] = useAsync(
    async (deviceUsage: DeviceUsage = 'mfa') => {
      // On successive runs with equivalent args, return the previously retrieved mfa challenge.
      const [prevChallenge, prevDeviceUsage] = getChallengeAttempt.data as [
        MfaAuthenticateChallenge,
        DeviceUsage,
      ];
      if (prevChallenge && deviceUsage == prevDeviceUsage) {
        return [prevChallenge, prevDeviceUsage] as const;
      }

      const challenge = await auth.getMfaChallenge({
        scope: props.challengeScope,
      });
      return [challenge, deviceUsage] as const;
    }
  );

  const [submitAttempt, submitWithMfa, setSubmitAttempt] = useAsync(
    async (
      mfaType?: DeviceType,
      deviceUsage?: DeviceUsage,
      totpCode?: string
    ) => {
      if (deviceUsage === 'passwordless') {
      }

      getChallenge(deviceUsage)
        .then(([[challenge], error]) => {
          // propagate getMfaChallenge errors to the mfaAttempt state
          if (error) throw error;
          return auth.getMfaChallengeResponse(challenge, mfaType, totpCode);
        })
        .then(props.onMfaResponse)
        .catch(err => {
          // make error user friendly.
          throw getReAuthenticationErrorMessage(err);
        })
        .finally(clearChallenge);
    }
  );

  function clearChallenge() {
    setGetChallengeAttempt(makeEmptyAttempt());
  }

  function clearSubmitAttempt() {
    setSubmitAttempt(makeEmptyAttempt());
  }

  return {
    getMfaChallenge,
    getChallengeAttempt,
    submitWithMfa,
    submitAttempt,
    clearSubmitAttempt,
  };
}

export type ReauthProps = {
  challengeScope: MfaChallengeScope;
  onMfaResponse(res: MfaChallengeResponse): void;
};

export type ReauthState = {
  getMfaChallenge: () => Promise<MfaAuthenticateChallenge>;
  getChallengeAttempt: Attempt<any>;
  submitWithMfa: (
    mfaType?: DeviceType,
    deviceUsage?: DeviceUsage,
    totpCode?: string
  ) => Promise<any>;
  submitAttempt: Attempt<void>;
  clearSubmitAttempt: () => void;
};

function getReAuthenticationErrorMessage(err) {
  if (err.message?.includes('attempt was made to use an object that is not')) {
    // Catch a webauthn frontend error that occurs on Firefox and replace it with a more helpful error message.
    return 'The two-factor device you used is not registered on this account. You must verify using a device that has already been registered.';
  }

  if (err.message === 'invalid totp token') {
    // This message relies on the status message produced by the auth server in
    // lib/auth/Server.checkOTP function. Please keep these in sync.
    return 'Invalid authenticator code';
  }

  return err;
}
