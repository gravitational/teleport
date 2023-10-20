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

import React from 'react';

import { render } from 'design/utils/testing';

import {
  LocalOnly,
  LocalWithOtp,
  LocalWithOptional,
  LocalWithWebauthn,
  Cloud,
  ServerError,
  PrimarySso,
  LocalDisabledWithSso,
  LocalDisabledNoSso,
} from './FormLogin.story';

test('auth2faType: off', () => {
  const { container } = render(<LocalOnly />);
  expect(container.firstChild).toMatchSnapshot();
});

test('auth2faType: otp rendering', () => {
  const { container } = render(<LocalWithOtp />);
  expect(container.firstChild).toMatchSnapshot();
});

test('auth2faType: webauthn rendering', () => {
  const { container } = render(<LocalWithWebauthn />);
  expect(container.firstChild).toMatchSnapshot();
});

test('auth2faType: optional rendering', () => {
  const { container } = render(<LocalWithOptional />);
  expect(container.firstChild).toMatchSnapshot();
});

test('cloud auth2faType: on rendering', () => {
  const { container } = render(<Cloud />);
  expect(container.firstChild).toMatchSnapshot();
});

test('server error rendering', () => {
  const { container } = render(<ServerError />);
  expect(container.firstChild).toMatchSnapshot();
});

test('sso providers rendering', () => {
  const { container } = render(<PrimarySso />);
  expect(container.firstChild).toMatchSnapshot();
});

test('sso list still renders when local auth is disabled', () => {
  const { container } = render(<LocalDisabledWithSso />);
  expect(container.firstChild).toMatchSnapshot();
});

test('no login enabled', () => {
  const { container } = render(<LocalDisabledNoSso />);
  expect(container.firstChild).toMatchSnapshot();
});
