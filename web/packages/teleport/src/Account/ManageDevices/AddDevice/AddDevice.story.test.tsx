/*
Copyright 2021-2022 Gravitational, Inc.

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
import { render, screen } from 'design/utils/testing';
import { LoadedWebauthn, Failed, QrCodeFailed } from './AddDevice.story';

test('render dialog to add a new mfa device with webauthn as preferred type', () => {
  render(<LoadedWebauthn />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('render failed state for dialog to add a new mfa device', () => {
  render(<Failed />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('render failed state for fetching QR Code for dialog to add a new mfa device', () => {
  render(<QrCodeFailed />);

  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
