import React from 'react';

import { render, screen } from 'design/utils/testing';

import { Loaded, LoadedWithPrompt, Failed } from './RequestReason.story';

test('loaded without custom prompt', () => {
  render(<Loaded />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('loaded with custom prompt', () => {
  render(<LoadedWithPrompt />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('failed', () => {
  render(<Failed />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
