import React from 'react';
import { Loaded, LoadedWithPrompt, Failed } from './RequestReason.story';
import { render, screen } from 'design/utils/testing';

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
