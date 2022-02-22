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
  const { container } = render(<FetchError />);
  expect(container).toMatchSnapshot();
});

test('connection error', () => {
  const { container } = render(<ConnectionError />);
  expect(container).toMatchSnapshot();
});

test('clipboard error', () => {
  const { container } = render(<ClipboardError />);
  expect(container).toMatchSnapshot();
});

test('unintended disconnect', () => {
  const { container } = render(<UnintendedDisconnect />);
  expect(container).toMatchSnapshot();
});

test('webauthn prompt', () => {
  const { container } = render(<WebAuthnPrompt />);
  expect(container).toMatchSnapshot();
});
