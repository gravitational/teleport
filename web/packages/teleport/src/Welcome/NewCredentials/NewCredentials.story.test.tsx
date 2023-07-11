/*
Copyright 2020-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { render } from 'design/utils/testing';

import * as story from './NewCredentials.story';

/**
 *
 * @remarks
 * This component is duplicated in Enterprise for Enterprise onboarding. If you are making edits to this file, check to see if the
 * equivalent change should be applied in Enterprise
 *
 */
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
test('story.SuccessAndPrivateKeyEnabledRegister', () => {
  const { container } = render(<story.SuccessAndPrivateKeyEnabledRegister />);
  expect(container.firstChild).toMatchSnapshot();
});
test('story.SuccessAndPrivateKeyEnabledReset', () => {
  const { container } = render(<story.SuccessAndPrivateKeyEnabledReset />);
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
