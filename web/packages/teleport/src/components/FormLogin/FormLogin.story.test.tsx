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
import {
  Off,
  Otp,
  Optional,
  Webauthn,
  Cloud,
  ServerError,
  SSOProviders,
  LocalAuthDisabled,
  LocalAuthDisabledNoSSO,
} from './FormLogin.story';
import { render } from 'design/utils/testing';

test('auth2faType: off', () => {
  const { container } = render(<Off />);
  expect(container.firstChild).toMatchSnapshot();
});

test('auth2faType: otp rendering', () => {
  const { container } = render(<Otp />);
  expect(container.firstChild).toMatchSnapshot();
});

test('auth2faType: webauthn rendering', () => {
  const { container } = render(<Webauthn />);
  expect(container.firstChild).toMatchSnapshot();
});

test('auth2faType: optional rendering', () => {
  const { container } = render(<Optional />);
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
  const { container } = render(<SSOProviders />);
  expect(container.firstChild).toMatchSnapshot();
});

test('nonrendering of user/pass/otp/login elements w/ local auth disabled', () => {
  const { container } = render(<LocalAuthDisabled />);
  expect(container.firstChild).toMatchSnapshot();
});

test('nonrendering of SSO providers w/ local auth disabled and no providers', () => {
  const { container } = render(<LocalAuthDisabledNoSSO />);
  expect(container.firstChild).toMatchSnapshot();
});
