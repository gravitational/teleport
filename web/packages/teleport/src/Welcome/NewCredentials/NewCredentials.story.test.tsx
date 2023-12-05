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

import * as story from './NewCredentials.story';

test('story.PasswordOnlyError', () => {
  const { container } = render(<story.PasswordOnlyError />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.PrimaryPasswordlessError', () => {
  const { container } = render(<story.PrimaryPasswordlessError />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.MfaDeviceError', () => {
  const { container } = render(<story.MfaDeviceError />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.MfaDeviceOtp', () => {
  const { container } = render(<story.MfaDeviceOtp />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.MfaDeviceWebauthn', () => {
  const { container } = render(<story.MfaDeviceWebauthn />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.MfaDeviceOn', () => {
  const { container } = render(<story.MfaDeviceOn />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.SuccessRegister', () => {
  const { container } = render(<story.SuccessRegister />);
  expect(container.firstChild).toMatchSnapshot();
});
test('story.SuccessReset', () => {
  const { container } = render(<story.SuccessReset />);
  expect(container.firstChild).toMatchSnapshot();
});
test('story.SuccessRegisterDashboard', () => {
  const { container } = render(<story.SuccessRegisterDashboard />);
  expect(container.firstChild).toMatchSnapshot();
});
test('story.SuccessResetDashboard', () => {
  const { container } = render(<story.SuccessResetDashboard />);
  expect(container.firstChild).toMatchSnapshot();
});
