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
