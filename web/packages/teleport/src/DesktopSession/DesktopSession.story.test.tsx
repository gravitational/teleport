import React from 'react';
import 'jest-canvas-mock';
import { render } from 'design/utils/testing';

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
  DismissibleError,
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
  const { getByTestId } = render(<InvalidProcessingState />);
  expect(getByTestId('Modal')).toMatchSnapshot();
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
  const { getByTestId } = render(<FetchError />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('connection error', () => {
  const { getByTestId } = render(<ConnectionError />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('unintended disconnect', () => {
  const { getByTestId } = render(<UnintendedDisconnect />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('dismissible error', () => {
  const { getByTestId } = render(<DismissibleError />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('webauthn prompt', () => {
  const { getByTestId } = render(<WebAuthnPrompt />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('another session active', () => {
  const { getByTestId } = render(<AnotherSessionActive />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});
