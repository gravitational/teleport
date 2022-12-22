/**
 * Copyright 2022 Gravitational, Inc.
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

import { Auth2faType, PreferredMfaType } from 'shared/services';

import createMfaOptions from './createMfaOptions';

describe('test retrieving mfa options', () => {
  const testCases: {
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
      expect: [],
    },
    {
      name: 'type on',
      type: 'on',
      expect: ['webauthn', 'otp'],
    },
    {
      name: 'type optional',
      type: 'optional',
      expect: ['webauthn', 'otp', 'optional'],
    },
    {
      name: 'type webauthn only',
      type: 'webauthn',
      expect: ['webauthn'],
    },
    {
      name: 'type otp only',
      type: 'otp',
      expect: ['otp'],
    },
  ];

  test.each(testCases)('$name', testCase => {
    const mfa = createMfaOptions({
      auth2faType: testCase.type,
    }).map(o => o.value);
    expect(mfa).toEqual(testCase.expect);
  });

  test('no "none" option if requireMfa=true', () => {
    const mfa = createMfaOptions({
      auth2faType: 'optional',
      required: true,
    }).map(o => o.value);
    expect(mfa).toEqual(['webauthn', 'otp']);
  });
});
