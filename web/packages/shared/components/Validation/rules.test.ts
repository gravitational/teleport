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

import {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
  requiredField,
  requiredRoleArn,
  requiredEmailLike,
  requiredIamRoleName,
} from './rules';

describe('requiredField', () => {
  const errMsg = 'error text';
  const validator = requiredField(errMsg);

  test.each`
    input                | expected
    ${'not empty value'} | ${{ valid: true, message: '' }}
    ${''}                | ${{ valid: false, message: errMsg }}
    ${null}              | ${{ valid: false, message: errMsg }}
  `('input with: $input', ({ input, expected }) => {
    expect(validator(input)()).toEqual(expected);
  });
});

describe('requiredToken', () => {
  const errMsg = 'Token is required';

  test.each`
    token           | expected
    ${'some token'} | ${{ valid: true }}
    ${''}           | ${{ valid: false, message: errMsg }}
    ${null}         | ${{ valid: false, message: errMsg }}
  `('token value with: $token', ({ token, expected }) => {
    expect(requiredToken(token)()).toEqual(expected);
  });
});

describe('requiredPassword', () => {
  const errMsg = 'Enter at least 12 characters';

  test.each`
    password            | expected
    ${'valid password'} | ${{ valid: true }}
    ${''}               | ${{ valid: false, message: errMsg }}
    ${null}             | ${{ valid: false, message: errMsg }}
  `('password value with: $password', ({ password, expected }) => {
    expect(requiredPassword(password)()).toEqual(expected);
  });
});

describe('requiredRoleArn', () => {
  test.each`
    roleArn                                                           | valid
    ${'arn:aws:iam::123456789012:role/some-role-name'}                | ${true}
    ${'arn:aws-otherpartition:iam::123456789012:role/some-role-name'} | ${true}
    ${'arn:aws:iam::123456789012:role/some/role/name'}                | ${false}
    ${'arn:aws:iam:123456789012:role/some-role-name'}                 | ${false}
    ${'arn:aws:iam::12345:role/some-role-name'}                       | ${false}
    ${'arn:iam:123456:role:some-role-name'}                           | ${false}
    ${'arn:aws:iam::123456789012:some-role-name'}                     | ${false}
    ${'arn:aws:iam::123456789012:role/'}                              | ${false}
    ${'arn:aws:iam::123456789012:role'}                               | ${false}
    ${''}                                                             | ${false}
    ${null}                                                           | ${false}
  `('role arn valid ($valid): $roleArn', ({ roleArn, valid }) => {
    const result = requiredRoleArn(roleArn)();
    expect(result.valid).toEqual(valid);
  });
});

describe('requiredIamRoleName', () => {
  test.each`
    roleArn                                | valid
    ${'some-role-name'}                    | ${true}
    ${'alphanum1234andspecialchars=.+-,'}  | ${true}
    ${'1'}                                 | ${true}
    ${Array.from('x'.repeat(64)).join('')} | ${true}
    ${Array.from('x'.repeat(65)).join('')} | ${false}
    ${null}                                | ${false}
    ${''}                                  | ${false}
  `('IAM role name valid ($valid): $roleArn', ({ roleArn, valid }) => {
    const result = requiredIamRoleName(roleArn)();
    expect(result.valid).toEqual(valid);
  });
});

describe('requiredConfirmedPassword', () => {
  const mismatchError = 'Password does not match';
  const confirmError = 'Please confirm your password';

  test.each`
    password          | confirmPassword   | expected
    ${'password1234'} | ${'password1234'} | ${{ valid: true }}
    ${''}             | ${'mismatch'}     | ${{ valid: false, message: mismatchError }}
    ${null}           | ${'mismatch'}     | ${{ valid: false, message: mismatchError }}
    ${'mistmatch'}    | ${null}           | ${{ valid: false, message: confirmError }}
    ${null}           | ${null}           | ${{ valid: false, message: confirmError }}
  `(
    'password: $password, confirmPassword: $confirmPassword',
    ({ password, confirmPassword, expected }) => {
      expect(requiredConfirmedPassword(password)(confirmPassword)()).toEqual(
        expected
      );
    }
  );
});

describe('requiredEmailLike', () => {
  test.each`
    email                  | expected
    ${''}                  | ${{ valid: false, kind: 'empty' }}
    ${'alice'}             | ${{ valid: false, kind: 'invalid' }}
    ${'alice@'}            | ${{ valid: false, kind: 'invalid' }}
    ${'@example.com'}      | ${{ valid: false, kind: 'invalid' }}
    ${'alice@example'}     | ${{ valid: true }}
    ${'alice@example.com'} | ${{ valid: true }}
  `('email: $email', ({ email, expected }) => {
    expect(requiredEmailLike(email)()).toEqual(
      expect.objectContaining(expected)
    );
  });
});
