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
