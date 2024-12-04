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

import { useState, useEffect } from 'react';

import { EventEmitterMfaSender } from 'teleport/lib/EventEmitterMfaSender';
import { TermEvent } from 'teleport/lib/term/enums';
import { parseMfaChallengeJson as parseMfaChallenge } from 'teleport/services/mfa/makeMfa';
import { DeviceType, MfaAuthenticateChallenge } from 'teleport/services/mfa';
import useReAuthenticate from 'teleport/components/ReAuthenticate/useReAuthenticate';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

export function useMfa(emitterSender: EventEmitterMfaSender): MfaState {
  const { attempt, submitWithMfa, mfaChallenge, setMfaChallenge } =
    useReAuthenticate({
      // challengeScope isn't actually used since we set the mfa
      // challenge manually, but it's a required arg.
      challengeScope: MfaChallengeScope.USER_SESSION,
      onMfaResponse: res => {
        emitterSender.sendChallengeResponse(res);
      },
    });

  const [mfaRequired, setMfaRequired] = useState(false);

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
    submitAttempt: attempt,
    submitWithMfa,
    mfaRequired,
  };
}

export type MfaState = {
  mfaChallenge: MfaAuthenticateChallenge;
  mfaRequired: boolean;
  submitAttempt: Attempt;
  submitWithMfa: (mfaType?: DeviceType, totp_code?: string) => Promise<void>;
};

// used for testing
export function makeDefaultMfaState(): MfaState {
  return {
    mfaChallenge: null,
    submitAttempt: {} as Attempt,
    mfaRequired: true,
    submitWithMfa: (mfaType?: DeviceType, totp_code?: string) => null,
  };
}
