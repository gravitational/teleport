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
