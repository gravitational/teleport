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
import 'jest-canvas-mock';
import { render, screen } from 'design/utils/testing';

import {
  BothProcessing,
  TdpProcessing,
  FetchProcessing,
  ConnectedSettingsFalse,
  ConnectedSettingsTrue,
  Disconnected,
  FetchError,
  TdpError,
  UnintendedDisconnect,
  WebAuthnPrompt,
  AnotherSessionActive,
} from './DesktopSession.story';

test('processing', () => {
  const { container } = render(<BothProcessing />);
  expect(container).toMatchSnapshot();
});

test('tdp processing', () => {
  const { container } = render(<TdpProcessing />);
  expect(container).toMatchSnapshot();
});

test('fetch processing', () => {
  const { container } = render(<FetchProcessing />);
  expect(container).toMatchSnapshot();
});

test('connected settings false', () => {
  const { container } = render(<ConnectedSettingsFalse />);
  expect(container).toMatchSnapshot();
});

test('connected settings true', () => {
  const { container } = render(<ConnectedSettingsTrue />);
  expect(container).toMatchSnapshot();
});

test('disconnected', () => {
  const { container } = render(<Disconnected />);
  expect(container).toMatchSnapshot();
});

test('fetch error', () => {
  render(<FetchError />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('connection error', () => {
  render(<TdpError />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('unintended disconnect', () => {
  render(<UnintendedDisconnect />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('webauthn prompt', () => {
  render(<WebAuthnPrompt />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('another session active', () => {
  render(<AnotherSessionActive />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
