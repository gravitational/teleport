import React from 'react';
import 'jest-canvas-mock';
import { render, screen } from 'design/utils/testing';

import {
  Processing,
  TdpProcessing,
  InvalidProcessingState,
  ConnectedSettingsFalse,
  ConnectedSettingsTrue,
  Disconnected,
  FetchError,
  ConnectionError,
  UnintendedDisconnect,
  WebAuthnPrompt,
  AnotherSessionActive,
} from './DesktopSession.story';

test('processing', () => {
  const { container } = render(<Processing />);
  expect(container).toMatchSnapshot();
});

test('tdp processing', () => {
  const { container } = render(<TdpProcessing />);
  expect(container).toMatchSnapshot();
});

test('invalid processing', () => {
  render(<InvalidProcessingState />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
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
  render(<ConnectionError />);
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
