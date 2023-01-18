import React from 'react';

import { render, screen } from 'design/utils/testing';

import { Loaded, Failed, Processing, Approved } from './RequestDelete.story';

test('loaded state', () => {
  render(<Loaded />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('failed state', () => {
  render(<Failed />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('processing state', () => {
  render(<Processing />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('approved role escalation', () => {
  render(<Approved />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
