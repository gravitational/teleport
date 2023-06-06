/**
 * Copyright 2020 Gravitational, Inc.
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

import {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
  requiredField,
  requiredRoleArn,
} from './rules';

describe('requiredField', () => {
  const errMsg = 'error text';
  const validator = requiredField(errMsg);

  test.each`
    input                | expected
    ${'not empty value'} | ${{ valid: true, message: '' }}
    ${''}                | ${{ valid: false, message: errMsg }}
    ${null}              | ${{ valid: false, message: errMsg }}
  `('test input with: $input', ({ input, expected }) => {
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
  `('test token value with: $token', ({ token, expected }) => {
    expect(requiredToken(token)()).toEqual(expected);
  });
});

describe('requiredPassword', () => {
  const errMsg = 'Enter at least 6 characters';

  test.each`
    password            | expected
    ${'valid password'} | ${{ valid: true }}
    ${''}               | ${{ valid: false, message: errMsg }}
    ${null}             | ${{ valid: false, message: errMsg }}
  `('test password value with: $password', ({ password, expected }) => {
    expect(requiredPassword(password)()).toEqual(expected);
  });
});

describe('requiredRoleArn', () => {
  test.each`
    roleArn                                      | valid
    ${'arn:aws:iam::123456:role/some-role-name'} | ${true}
    ${'arn:aws:iam::123456:role:some-role-name'} | ${true}
    ${'arn:aws:iam:123456:role:some-role-name'}  | ${true}
    ${'arn:iam:123456:role:some-role-name'}      | ${false}
    ${'arn:aws:iam:123456:some-role-name'}       | ${false}
    ${'arn:aws:123456:role:some-role-name'}      | ${false}
    ${''}                                        | ${false}
    ${null}                                      | ${false}
  `('test valid role arn: $roleArn', ({ roleArn, valid }) => {
    const result = requiredRoleArn(roleArn)();
    expect(result.valid).toEqual(valid);
  });
});

describe('requiredConfirmedPassword', () => {
  const mismatchError = 'Password does not match';
  const confirmError = 'Please confirm your password';

  test.each`
    password       | confirmPassword | expected
    ${'pwd123'}    | ${'pwd123'}     | ${{ valid: true }}
    ${''}          | ${'mismatch'}   | ${{ valid: false, message: mismatchError }}
    ${null}        | ${'mismatch'}   | ${{ valid: false, message: mismatchError }}
    ${'mistmatch'} | ${null}         | ${{ valid: false, message: confirmError }}
    ${null}        | ${null}         | ${{ valid: false, message: confirmError }}
  `(
    'test password: $password, confirmPassword: $confirmPassword',
    ({ password, confirmPassword, expected }) => {
      expect(requiredConfirmedPassword(password)(confirmPassword)()).toEqual(
        expected
      );
    }
  );
});
