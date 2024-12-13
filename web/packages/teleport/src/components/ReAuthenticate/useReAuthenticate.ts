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

import { useCallback, useEffect, useState } from 'react';
import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

import {
  DeviceType,
  DeviceUsage,
  getMfaChallengeOptions,
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
  MfaOption,
} from 'teleport/services/mfa';

export default function useReAuthenticate({
  challengeScope,
  onMfaResponse,
}: ReauthProps): ReauthState {
  const [challengeState, setChallengeState] = useState<challengeState>();

  const [getChallengeAttempt, getChallenge] = useAsync(
    useCallback(
      async (deviceUsage: DeviceUsage = 'mfa') => {
        // If the challenge state is empty, used, or has different args,
        // retrieve a new mfa challenge and set it in the state.
        if (!challengeState || challengeState.deviceUsage != deviceUsage) {
          console.log('new challenge!');
          const challenge = await auth.getMfaChallenge({
            scope: challengeScope,
          });
          setChallengeState({
            challenge,
            deviceUsage,
            mfaOptions: getMfaChallengeOptions(challenge),
            used: false,
          });
        }

        return challengeState.challenge;
      },
      [challengeState, setChallengeState, challengeScope]
    )
  );

  useEffect(() => {
    getChallenge();
  }, [getChallenge]);

  const [submitAttempt, submitWithMfa, setSubmitAttempt] = useAsync(
    useCallback(
      async (
        mfaType?: DeviceType,
        deviceUsage?: DeviceUsage,
        totpCode?: string
      ) => {
        const [challenge, err] = await getChallenge(deviceUsage);
        if (err) throw err;

        const response = auth.getMfaChallengeResponse(
          challenge,
          mfaType,
          totpCode
        );
        try {
          await response;
        } catch (err) {
          throw new Error(getReAuthenticationErrorMessage(err));
        }

        try {
          return onMfaResponse(await response);
        } finally {
          setChallengeState({
            ...challengeState,
            used: true,
          });
        }
      },
      [getChallenge, onMfaResponse, challengeState, setChallengeState]
    )
  );

  function clearSubmitAttempt() {
    setSubmitAttempt(makeEmptyAttempt());
  }

  return {
    challengeState,
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
  challengeState: challengeState;
  getChallengeAttempt: Attempt<any>;
  submitWithMfa: (
    mfaType?: DeviceType,
    deviceUsage?: DeviceUsage,
    totpCode?: string
  ) => Promise<[void, Error]>;
  submitAttempt: Attempt<void>;
  clearSubmitAttempt: () => void;
};

type challengeState = {
  challenge: MfaAuthenticateChallenge;
  deviceUsage: DeviceUsage;
  mfaOptions: MfaOption[];
  used: boolean;
};

function getReAuthenticationErrorMessage(err: Error | string): string {
  const errMsg = err instanceof Error ? err.message : err;

  if (errMsg.includes('attempt was made to use an object that is not')) {
    // Catch a webauthn frontend error that occurs on Firefox and replace it with a more helpful error message.
    return 'The two-factor device you used is not registered on this account. You must verify using a device that has already been registered.';
  }

  if (errMsg === 'invalid totp token') {
    // This message relies on the status message produced by the auth server in
    // lib/auth/Server.checkOTP function. Please keep these in sync.
    return 'Invalid authenticator code';
  }

  return errMsg;
}
