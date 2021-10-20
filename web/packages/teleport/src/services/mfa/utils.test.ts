/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { getMfaOptions } from './utils';
import { Auth2faType, PreferredMfaType } from 'shared/services';

describe('test retrieving mfa options', () => {
  const tests: {
    name: string;
    type?: Auth2faType;
    preferred?: PreferredMfaType;
    expect: Auth2faType[];
  }[] = [
    {
      name: 'type undefined',
      expect: [],
    },
    {
      name: 'type off',
      type: 'off',
      preferred: 'u2f',
      expect: [],
    },
    {
      name: 'type on with u2f preferred',
      type: 'on',
      preferred: 'u2f',
      expect: ['u2f', 'otp'],
    },
    {
      name: 'type on with webauthn preferred',
      type: 'on',
      preferred: 'webauthn',
      expect: ['webauthn', 'otp'],
    },
    {
      name: 'type optional with u2f preferred',
      type: 'optional',
      preferred: 'u2f',
      expect: ['u2f', 'otp', 'optional'],
    },
    {
      name: 'type optional with webauthn preferred',
      type: 'optional',
      preferred: 'webauthn',
      expect: ['webauthn', 'otp', 'optional'],
    },
    {
      name: 'type u2f only',
      type: 'u2f',
      preferred: 'webauthn',
      expect: ['u2f'],
    },
    {
      name: 'type webauthn only',
      type: 'webauthn',
      preferred: 'u2f',
      expect: ['webauthn'],
    },
    {
      name: 'type otp only',
      type: 'otp',
      preferred: 'webauthn',
      expect: ['otp'],
    },
  ];

  tests.forEach(tc => {
    test(`${tc.name}`, () => {
      const vals = getMfaOptions(tc.type, tc.preferred).map(o => o.value);
      // Order matters.
      expect(vals).toEqual(tc.expect);
    });
  });
});
