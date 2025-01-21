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

import { useCallback, useEffect, useRef, useState } from 'react';

import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { EventEmitterMfaSender } from 'teleport/lib/EventEmitterMfaSender';
import { TermEvent } from 'teleport/lib/term/enums';
import {
  CreateAuthenticateChallengeRequest,
  parseMfaChallengeJson,
} from 'teleport/services/auth';
import auth from 'teleport/services/auth/auth';
import {
  DeviceType,
  getMfaChallengeOptions,
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
  MfaOption,
} from 'teleport/services/mfa';

export type MfaProps = {
  req?: CreateAuthenticateChallengeRequest;
  isMfaRequired?: boolean | null;
};

type mfaResponsePromiseWithResolvers = {
  promise: Promise<MfaChallengeResponse>;
  resolve: (v: MfaChallengeResponse) => void;
  reject: (err: Error) => void;
};

/**
 * Use the returned object to request MFA checks with a shared state.
 * When MFA authentication is in progress, the object's properties can
 * be used to display options to the user and prompt for them to complete
 * the MFA check.
 */
export function useMfa({ req, isMfaRequired }: MfaProps): MfaState {
  const [mfaRequired, setMfaRequired] = useState<boolean>();
  const [options, setMfaOptions] = useState<MfaOption[]>();
  const [challenge, setMfaChallenge] = useState<MfaAuthenticateChallenge>();

  useEffect(() => {
    setMfaRequired(isMfaRequired);
  }, [isMfaRequired]);

  useEffect(() => {
    setMfaRequired(null);
  }, [req?.isMfaRequiredRequest]);

  // getResponse is used to initiate MFA authentication.
  //   1. Check if MFA is required by getting a new MFA challenge
  //   2. If MFA is required, set the challenge in the MFA state and wait for it to
  //      be resolved by the caller.
  //   3. The caller sees the mfa challenge set in state and submits an mfa response
  //      request with arguments provided by the user (mfa type, otp code).
  //   4. Receive the mfa response through the mfaResponsePromise ref and return it.
  //
  // The caller should also display errors seen in attempt.
  const [attempt, getResponse, setMfaAttempt] = useAsync(
    useCallback(
      async (challenge?: MfaAuthenticateChallenge) => {
        // If a previous call determined that MFA is not required, this is a noop.
        if (mfaRequired === false) return;

        challenge = challenge ? challenge : await auth.getMfaChallenge(req);
        if (!challenge) {
          setMfaRequired(false);
          return;
        }

        // Prepare a new promise to collect the mfa response retrieved
        // through the submit function.
        let resolve: (value: MfaChallengeResponse) => void;
        let reject: (err: Error) => void;
        const promise = new Promise<MfaChallengeResponse>((res, rej) => {
          resolve = res;
          reject = rej;
        });

        mfaResponseRef.current = {
          promise,
          resolve,
          reject,
        };

        // Set mfa requirement and options after we get a challenge for the first time.
        setMfaRequired(true);
        setMfaOptions(getMfaChallengeOptions(challenge));

        setMfaChallenge(challenge);
        try {
          return await promise;
        } finally {
          setMfaChallenge(null);
        }
      },
      [req, mfaRequired]
    )
  );

  const mfaResponseRef = useRef<mfaResponsePromiseWithResolvers>();

  const cancelAttempt = () => {
    if (mfaResponseRef.current) {
      mfaResponseRef.current.reject(new MfaCanceledError());
    }
  };

  const getChallengeResponse = useCallback(
    async (challenge?: MfaAuthenticateChallenge) => {
      const [resp, err] = await getResponse(challenge);

      if (err) throw err;

      return resp;
    },
    [getResponse]
  );

  const submit = useCallback(
    async (mfaType?: DeviceType, totpCode?: string) => {
      if (!mfaResponseRef.current) {
        throw new Error('submit called without an in flight MFA attempt');
      }

      try {
        await mfaResponseRef.current.resolve(
          await auth.getMfaChallengeResponse(challenge, mfaType, totpCode)
        );
      } catch (err) {
        setMfaAttempt({
          data: null,
          status: 'error',
          statusText: err.message,
          error: err,
        });
      }
    },
    [challenge, setMfaAttempt]
  );

  return {
    required: mfaRequired,
    options,
    challenge,
    getChallengeResponse,
    submit,
    attempt,
    cancelAttempt,
  };
}

export function useMfaEmitter(emitterSender: EventEmitterMfaSender): MfaState {
  const [mfaRequired, setMfaRequired] = useState(false);

  const mfa = useMfa({ isMfaRequired: mfaRequired });

  useEffect(() => {
    const challengeHandler = async (challengeJson: string) => {
      // set Mfa required for other uses of this MfaState (e.g. file transfers)
      setMfaRequired(true);

      const challenge = parseMfaChallengeJson(JSON.parse(challengeJson));
      const resp = await mfa.getChallengeResponse(challenge);
      emitterSender.sendChallengeResponse(resp);
    };

    emitterSender?.on(TermEvent.MFA_CHALLENGE, challengeHandler);
    return () => {
      emitterSender?.removeListener(TermEvent.MFA_CHALLENGE, challengeHandler);
    };
  }, [mfa, emitterSender]);

  return mfa;
}

export type MfaState = {
  required: boolean;
  options: MfaOption[];
  challenge: MfaAuthenticateChallenge;
  // Generally you wouldn't pass in a challenge, unless you already
  // have one handy, e.g. from a terminal websocket message.
  getChallengeResponse: (
    challenge?: MfaAuthenticateChallenge
  ) => Promise<MfaChallengeResponse>;
  submit: (mfaType?: DeviceType, totpCode?: string) => Promise<void>;
  attempt: Attempt<any>;
  cancelAttempt: () => void;
};

// used for testing
export function makeDefaultMfaState(): MfaState {
  return {
    required: true,
    options: null,
    challenge: null,
    getChallengeResponse: async () => null,
    submit: () => null,
    attempt: makeEmptyAttempt(),
    cancelAttempt: () => null,
  };
}

export class MfaCanceledError extends Error {
  constructor() {
    super('User canceled MFA attempt');
  }
}
