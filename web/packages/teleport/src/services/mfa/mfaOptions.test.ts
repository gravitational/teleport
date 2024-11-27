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

import { Auth2faType } from 'shared/services';
import { getMfaChallengeOptions, getMfaRegisterOptions } from './mfaOptions';
import { DeviceType, MfaAuthenticateChallenge } from './types';
import { SSOChallenge } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

describe('test retrieving mfa options from Auth2faType', () => {
  const testCases: {
    name: string;
    type?: Auth2faType;
    expect: DeviceType[];
  }[] = [
    {
      name: 'type undefined',
      expect: [],
    },
    {
      name: 'type on',
      type: 'on',
      expect: ['webauthn', 'totp'],
    },
    {
      name: 'type webauthn only',
      type: 'webauthn',
      expect: ['webauthn'],
    },
    {
      name: 'type otp only',
      type: 'otp',
      expect: ['totp'],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const mfa = getMfaRegisterOptions(testCase.type).map(o => o.value);
    expect(mfa).toEqual(testCase.expect);
  });
});

describe('test retrieving mfa options from MFA Challenge', () => {
  const testCases: {
    name: string;
    challenge?: MfaAuthenticateChallenge;
    expect: DeviceType[];
  }[] = [
    {
      name: 'type undefined',
      expect: [],
    },
    {
      name: 'challenge totp',
      challenge: {
        totpChallenge: true,
      },
      expect: ['totp'],
    },
    {
      name: 'challenge webauthn',
      challenge: {
        webauthnPublicKey: Object.create(PublicKeyCredential),
      },
      expect: ['webauthn'],
    },
    {
      name: 'challenge sso',
      challenge: {
        ssoChallenge: Object.create(SSOChallenge),
      },
      expect: ['webauthn', 'totp'],
    },
    {
      name: 'challenge all',
      challenge: {
        totpChallenge: true,
        webauthnPublicKey: Object.create(PublicKeyCredential),
        ssoChallenge: Object.create(SSOChallenge),
      },
      expect: ['webauthn', 'totp'],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const mfa = getMfaChallengeOptions(testCase.challenge).map(o => o.value);
    expect(mfa).toEqual(testCase.expect);
  });
});
