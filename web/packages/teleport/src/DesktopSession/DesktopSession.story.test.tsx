import React from 'react';
import 'jest-canvas-mock';
import { render } from 'design/utils/testing';

import {
  ConnectedSettingsFalse,
  ConnectedSettingsTrue,
  Disconnected,
  FetchError,
  ConnectionError,
  ClipboardError,
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
  const { getByTestId } = render(<FetchError />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('connection error', () => {
  const { getByTestId } = render(<ConnectionError />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('clipboard error', () => {
  const { getByTestId } = render(<ClipboardError />);
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
