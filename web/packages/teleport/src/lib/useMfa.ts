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

// useMfa prepares an MFA state designed for use with AuthnDialog.
// 1. Call getMfaChallengeResponse() where MFA may be required.
//    If MFA is not required, it should be a noop, returning null.
// 2. Setup AuthnDialog using the MFA state, to show a dialog when
//    mfaAttempt.state === 'processing'.
export function useMfa({ req, isMfaRequired }: MfaProps): MfaState {
  const [mfaRequired, setMfaRequired] = useState<boolean>();
  const [options, setMfaOptions] = useState<MfaOption[]>();

  const [challenge, setMfaChallenge] = useState<MfaAuthenticateChallenge>();
  const mfaResponsePromise =
    useRef<PromiseWithResolvers<MfaChallengeResponse>>();

  useEffect(() => {
    setMfaRequired(isMfaRequired);
  }, [isMfaRequired]);

  useEffect(() => {
    setMfaRequired(null);
  }, [req?.isMfaRequiredRequest]);

  const [attempt, getResponse, setMfaAttempt] = useAsync(
    useCallback(
      async (challenge?: MfaAuthenticateChallenge) => {
        if (!challenge) {
          if (mfaRequired === false) return;

          challenge = await auth.getMfaChallenge(req);
          if (!challenge) {
            setMfaRequired(false);
            return;
          }
        }

        // Set mfa requirement and options after we get a challenge for the first time.
        if (!mfaRequired) setMfaRequired(true);
        if (!options) setMfaOptions(getMfaChallengeOptions(challenge));

        mfaResponsePromise.current = Promise.withResolvers();
        setMfaChallenge(challenge);
        try {
          return await mfaResponsePromise.current.promise;
        } finally {
          mfaResponsePromise.current = null;
          setMfaChallenge(null);
        }
      },
      [req, mfaResponsePromise, options, mfaRequired]
    )
  );

  const resetAttempt = () => {
    if (mfaResponsePromise.current) mfaResponsePromise.current.reject();
    mfaResponsePromise.current = null;
    setMfaChallenge(null);
    setMfaAttempt(makeEmptyAttempt());
  };

  const getChallengeResponse = async (challenge?: MfaAuthenticateChallenge) => {
    const [resp, err] = await getResponse(challenge);
    if (err) throw err;
    return resp;
  };

  const submit = useCallback(
    async (mfaType?: DeviceType, totpCode?: string) => {
      try {
        const resp = await auth.getMfaChallengeResponse(
          challenge,
          mfaType,
          totpCode
        );
        mfaResponsePromise.current.resolve(resp);
      } catch (err) {
        setMfaAttempt({
          data: null,
          status: 'error',
          statusText: err.message,
          error: err,
        });
      }
    },
    [challenge, mfaResponsePromise, setMfaAttempt]
  );

  return {
    required: mfaRequired,
    options,
    challenge,
    getChallengeResponse,
    submit,
    attempt,
    resetAttempt,
  };
}

export function useMfaTty(emitterSender: EventEmitterMfaSender): MfaState {
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
  resetAttempt: () => void;
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
    resetAttempt: () => null,
  };
}
