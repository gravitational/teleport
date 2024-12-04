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

import { EventEmitterMfaSender } from 'teleport/lib/EventEmitterMfaSender';
import { TermEvent } from 'teleport/lib/term/enums';
import auth from 'teleport/services/auth/auth';
import { DeviceType, MfaAuthenticateChallenge } from 'teleport/services/mfa';
import { parseMfaChallengeJson as parseMfaChallenge } from 'teleport/services/mfa/makeMfa';

export function useMfa(emitterSender: EventEmitterMfaSender): MfaState {
  const [mfaChallenge, setMfaChallenge] = useState<MfaAuthenticateChallenge>();
  const [mfaRequired, setMfaRequired] = useState(false);

  const [submitAttempt, submitMfa] = useAsync(
    useCallback(
      async (mfaType: DeviceType) => {
        if (!mfaChallenge)
          throw new Error('expected non empty mfa challenge in state');

        return auth
          .getMfaChallengeResponse(mfaChallenge, mfaType)
          .then(res => emitterSender.sendChallengeResponse(res));
      },
      [mfaChallenge]
    )
  );

  useEffect(() => {
    const challengeHandler = (challengeJson: string) => {
      const challenge = JSON.parse(challengeJson);
      setMfaChallenge(parseMfaChallenge(challenge));
      setMfaRequired(true);
    };

    emitterSender?.on(TermEvent.MFA_CHALLENGE, challengeHandler);
    return () => {
      emitterSender?.removeListener(TermEvent.MFA_CHALLENGE, challengeHandler);
    };
  }, [emitterSender]);

  return {
    mfaChallenge,
    submitAttempt,
    submitMfa,
    mfaRequired,
  };
}

export type MfaState = {
  mfaChallenge: MfaAuthenticateChallenge;
  submitMfa: (mfaType?: DeviceType) => Promise<any>;
  submitAttempt: Attempt<void>;
  mfaRequired: boolean;
};

// used for testing
export function makeDefaultMfaState(): MfaState {
  return {
    mfaChallenge: null,
    submitAttempt: makeEmptyAttempt(),
    submitMfa: () => null,
    mfaRequired: false,
  };
}
