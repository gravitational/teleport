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

test('story.off', () => {
  const { container } = render(<story.MfaOff />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.Otp', () => {
  const { container } = render(<story.MfaOtp />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.OtpError', () => {
  const { container } = render(<story.MfaOtpError />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.Webauthn', () => {
  const { container } = render(<story.MfaWebauthn />);
  expect(container.firstChild).toMatchSnapshot();
});

test('story.Webauthn Error', () => {
  const { container } = render(<story.MfaWebauthnError />);
  expect(container.firstChild).toMatchSnapshot();
});
