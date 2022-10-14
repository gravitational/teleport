import React from 'react';
import 'jest-canvas-mock';
import { render, screen } from 'design/utils/testing';

import {
  ConnectedSettingsFalse,
  ConnectedSettingsTrue,
  Disconnected,
  FetchError,
  ConnectionError,
  UnintendedDisconnect,
  WebAuthnPrompt,
  DismissibleError,
} from './DesktopSession.story';

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
  render(<ConnectionError />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('unintended disconnect', () => {
  render(<UnintendedDisconnect />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('dismissible error', () => {
  render(<DismissibleError />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('webauthn prompt', () => {
  render(<WebAuthnPrompt />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
